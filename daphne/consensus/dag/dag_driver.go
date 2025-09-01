package dag

import (
	"bytes"
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

// Factory defines the configuration for the dag consensus algorithm instance.
type Factory struct {
	EmitInterval    time.Duration
	Creator         model.CreatorId
	Committee       *consensus.Committee
	LayeringFactory layering.Factory
}

// NewActive creates a new active dag consensus instance.
// source is used to get candidate transactions for event emission.
func (f Factory) NewActive(server p2p.Server,
	source consensus.TransactionProvider) consensus.Consensus {
	return newActiveDagConsensus(server, f.LayeringFactory.NewLayering(f.Committee), f.Creator, source, f.EmitInterval)
}

// NewPassive creates a new passive dag consensus instance.
// This instance does not create/emit events but listens for them
// from the active instances and reproduces the DAG.
// The reproduced DAG is used to linearize events and their respective transactions.
func (f Factory) NewPassive(server p2p.Server) consensus.Consensus {
	return newPassiveDagConsensus(server, f.LayeringFactory.NewLayering(f.Committee))
}

// Consensus is responsible for coordinating the consensus process, broadcasting new
// events, handling incoming event messages from peers, maintaining the DAG,
// and linearizing the events based on an assigned layering.
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
	seenEventsMutex      sync.Mutex
}

func newActiveDagConsensus(
	server p2p.Server,
	layering layering.Layering,
	creator model.CreatorId,
	transactionProvider consensus.TransactionProvider,
	emitInterval time.Duration,
) *Consensus {
	consensus := newPassiveDagConsensus(server, layering)
	consensus.creator = creator
	consensus.emitter = generic.StartEmitter(
		&emissionPayloadSourceAdapter{consensus: consensus, transactionSource: transactionProvider},
		consensus.gossip,
		emitInterval,
	)

	return consensus
}

func newPassiveDagConsensus(
	server p2p.Server,
	layering layering.Layering,
) *Consensus {
	consensus := &Consensus{
		layering:   layering,
		dag:        model.NewDag(),
		seenEvents: make(map[model.EventId]struct{}),
	}
	gossip := generic.NewGossip(
		server,
		func(msg model.EventMessage) model.EventId { return msg.EventId() },
		p2p.MessageCode_DagConsensus_NewEvent,
	)
	gossip.RegisterReceiver(&onMessageAdapter{consensus: consensus})
	consensus.gossip = gossip

	return consensus
}

// RegisterListener registers a new bundle listener to receive notifications
// about new bundles emitted by the dag consensus algorithm.
func (c *Consensus) RegisterListener(listener consensus.BundleListener) {
	if listener != nil {
		c.eventProcessingMutex.Lock()
		defer c.eventProcessingMutex.Unlock()
		c.listeners = append(c.listeners, listener)
	}
}

// Stop stops the active DAG consensus instance and its event emission.
// It blocks until the emission loop exits.
func (c *Consensus) Stop() {
	if c.emitter != nil {
		c.emitter.Stop()
		c.emitter = nil
	}
}

func (c *Consensus) processEventMessage(msg model.EventMessage) {
	c.seenEventsMutex.Lock()
	if _, alreadyProcessed := c.seenEvents[msg.EventId()]; alreadyProcessed {
		c.seenEventsMutex.Unlock()
		return
	}
	c.seenEvents[msg.EventId()] = struct{}{}
	c.seenEventsMutex.Unlock()

	// DAG can be updated in parallel with candidate/leader processing.
	connected := c.dag.AddEvent(msg)

	c.eventProcessingMutex.Lock()
	defer c.eventProcessingMutex.Unlock()

	for _, event := range connected {
		if c.layering.IsCandidate(event) {
			c.leaderCandidates = append(c.leaderCandidates, event)
		}
	}

	newLeaders := []*model.Event{}
	// Look for new leaders and remove events that lost the candidate status.
	c.leaderCandidates = slices.DeleteFunc(
		c.leaderCandidates,
		func(candidate *model.Event) bool {
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
		},
	)

	newLeaders = c.layering.SortLeaders(c.dag, newLeaders)
	// Deliver respective delta closures of each leader, in a deterministic manner.
	for _, leader := range newLeaders {
		prevCovered := c.lastDecided.GetClosure()
		newCovered := leader.GetClosure()

		maps.DeleteFunc(newCovered, func(e *model.Event, _ struct{}) bool {
			_, exists := prevCovered[e]
			return exists
		})
		// TODO: make layering contribute to sorting or find a better universal way.
		sortedClosure := slices.SortedFunc(maps.Keys(newCovered), func(a, b *model.Event) int {
			return bytes.Compare(a.EventId().Serialize(), b.EventId().Serialize())
		})
		c.deliverConfirmedEvents(sortedClosure)

		c.lastDecided = leader
	}
}

// deliverConfirmedEvents bundles transactions from events in their
// respective order and delivers them to registered bundle listeners.
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
