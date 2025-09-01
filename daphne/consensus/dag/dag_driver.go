package dag

import (
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// Factory defines the configuration for the central consensus algorithm
// instance.
type Factory struct {
	EmitInterval    time.Duration
	Creator         model.CreatorId
	Committee       *consensus.Committee
	LayeringFactory layering.Factory
}

// NewActive creates a new active central consensus instance.
// source is used to get candidate transactions for the next bundle.
func (f Factory) NewActive(server p2p.Server,
	source consensus.TransactionProvider) consensus.Consensus {
	return newActiveDagConsensus(server, f.LayeringFactory.NewLayering(f.Committee), f.Creator, source, f.EmitInterval)
}

// NewPassive creates a new passive central consensus instance.
// This instance does not create/emit bundles but listens for them
// from the leader.
func (f Factory) NewPassive(server p2p.Server) consensus.Consensus {
	return newPassiveDagConsensus(server, f.LayeringFactory.NewLayering(f.Committee))
}

// Consensus implements the central consensus algorithm.
// It is responsible for coordinating the consensus process, broadcasting new
// bundles, and handling incoming bundle messages from peers.
type Consensus struct {
	creator  model.CreatorId
	dag      *model.Dag
	layering layering.Layering

	leaderCandidates []*model.Event
	lastDecided      *model.Event
	nextBundleNumber uint32

	seenEvents map[model.EventId]struct{}
	gossip     generic.Gossip[model.EventMessage]
	emitter    *generic.Emitter[model.EventMessage]

	listeners []consensus.BundleListener

	eventProcessingMutex sync.Mutex
}

func newActiveDagConsensus(
	server p2p.Server,
	layering layering.Layering,
	creator model.CreatorId,
	transactionProvider consensus.TransactionProvider,
	emitInterval time.Duration,
) *Consensus {
	res := newPassiveDagConsensus(server, layering)
	res.creator = creator
	res.emitter = generic.StartEmitter(
		&emissionPayloadSourceAdapter{consensus: res, transactionSource: transactionProvider},
		res.gossip,
		emitInterval,
	)

	return res
}

// newPassiveDagConsensus creates a new passive central consensus instance.
// This instance does not emit bundles but listens for them from the leader.
func newPassiveDagConsensus(
	server p2p.Server,
	layering layering.Layering,
) *Consensus {
	gossip := generic.NewGossip(
		server,
		func(msg model.EventMessage) model.EventId { return msg.EventId() },
		p2p.MessageCode_DagConsensus_NewEvent,
	)
	c := &Consensus{
		layering:   layering,
		dag:        model.NewDag(),
		seenEvents: make(map[model.EventId]struct{}),
	}

	gossip.RegisterReceiver(&onMessageAdapter{consensus: c})
	c.gossip = gossip

	return c
}

// RegisterListener registers a new bundle listener to receive notifications
// about new bundles emitted by the central consensus algorithm's leader.
func (c *Consensus) RegisterListener(listener consensus.BundleListener) {
	if listener != nil {
		c.eventProcessingMutex.Lock()
		defer c.eventProcessingMutex.Unlock()
		c.listeners = append(c.listeners, listener)
	}
}

// Stop stops the active dag consensus instance.
// It closes the quit channel to signal the emitter goroutine to stop,
// and waits for the done channel to be closed before returning.
func (c *Consensus) Stop() {
	if c.emitter != nil {
		c.emitter.Stop()
		c.emitter = nil
	}
}

func (c *Consensus) processEventMessage(msg model.EventMessage) {
	c.eventProcessingMutex.Lock()
	defer c.eventProcessingMutex.Unlock()

	if _, alreadyProcessed := c.seenEvents[msg.EventId()]; alreadyProcessed {
		return
	}
	c.seenEvents[msg.EventId()] = struct{}{}

	connected := c.dag.AddEvent(msg)
	for _, event := range connected {
		c.gossip.Broadcast(event.ToEventMessage())
	}
	for _, event := range connected {
		if c.layering.IsCandidate(event) {
			c.leaderCandidates = append(c.leaderCandidates, event)
		}
	}

	newLeaders := []*model.Event{}
	shouldRemoveFromCandidates := func(candidate *model.Event) bool {
		isLeader := c.layering.IsLeader(c.dag, candidate)
		switch isLeader {
		case layering.VerdictYes:
			newLeaders = append(newLeaders, candidate)
			return true
		case layering.VerdictNo:
			return true
		default:
			return false
		}
	}
	c.leaderCandidates = slices.DeleteFunc(c.leaderCandidates, shouldRemoveFromCandidates)

	if len(newLeaders) == 0 {
		return
	}
	newLeaders = c.layering.SortLeaders(c.dag, newLeaders)

	// Deliver respective delta closures in sorted order
	for _, leader := range newLeaders {
		prevCovered := c.lastDecided.GetClosure()
		newCovered := leader.GetClosure()
		maps.DeleteFunc(newCovered, func(e *model.Event, _ struct{}) bool {
			_, exists := prevCovered[e]
			return exists
		})
		// TODO: sort events in a deterministic manner.
		sortedClosure := slices.SortedFunc(maps.Keys(newCovered), func(a, b *model.Event) int {
			if a.Seq() != b.Seq() {
				return int(a.Seq()) - int(b.Seq())
			}
			return int(a.Creator()) - int(b.Creator())
		})
		c.deliverConfirmedEvents(sortedClosure)

		c.lastDecided = leader
	}
}

// deliverConfirmedEvents bundles transactions from events in a provided respective order,
// bundles them and delivers to registered bundle listeners.
func (c *Consensus) deliverConfirmedEvents(events []*model.Event) {
	transactions := []types.Transaction{}
	for _, event := range events {
		transactions = append(transactions, event.Payload()...)
	}
	c.nextBundleNumber++
	bundle := types.Bundle{
		Number:       c.nextBundleNumber,
		Transactions: transactions,
	}
	for _, listener := range c.listeners {
		listener.OnNewBundle(bundle)
	}
}

func (c *Consensus) createNewEvent(transactions []types.Transaction) model.EventMessage {
	dagHeads := c.dag.GetHeads()

	parents := []model.EventId{}
	if _, found := dagHeads[c.creator]; found {
		parents = []model.EventId{dagHeads[c.creator].EventId()}
		for creator, tip := range dagHeads {
			if creator != c.creator {
				parents = append(parents, tip.EventId())
			}
		}
	}
	eventMessage := model.EventMessage{
		Creator: c.creator,
		Parents: parents,
		Payload: transactions,
	}
	c.processEventMessage(eventMessage)

	return eventMessage
}

type emissionPayloadSourceAdapter struct {
	consensus         *Consensus
	transactionSource consensus.TransactionProvider
}

func (c *emissionPayloadSourceAdapter) GetEmissionPayload() model.EventMessage {
	return c.consensus.createNewEvent(c.transactionSource.GetCandidateTransactions())
}

// onMessageAdapter is an adapter that implements the p2p.MessageHandler
// interface for the Central consensus algorithm.
type onMessageAdapter struct {
	consensus *Consensus
}

// OnMessage is called by the gossip protocol when a new message is receivec.
// It delegates the handling of the message to the central consensus instance.
func (m *onMessageAdapter) OnMessage(msg model.EventMessage) {
	m.consensus.processEventMessage(msg)
}
