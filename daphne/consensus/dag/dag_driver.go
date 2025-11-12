package dag

import (
	"bytes"
	"slices"
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/emitter"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

// Factory defines the configuration for the DAG consensus algorithm instance:
//   - EmitInterval: the interval at which events are created and gossiped (Active instance only).
//   - Creator: the ID of the creator of the events (Active instance only).
//   - Committee: the creator committee which participates in DAG building and [layering.Layering].
//   - LayeringFactory: the factory configuration used to instantiate the layering algorithm.
type Factory struct {
	EmitInterval    time.Duration
	Committee       *consensus.Committee
	LayeringFactory layering.Factory
}

// NewActive creates a new active DAG consensus instance parametrized by the factory configuration.
// The active instance produced by NewActive extends the responsibility of the instance
// created by [Factory.NewPassive], by creating and periodically emitting DAG events.
// The source is used to get candidate transactions for event emission, and the provided
// server is used for P2P communication.
func (f Factory) NewActive(
	server p2p.Server,
	creator consensus.ValidatorId,
	source consensus.TransactionProvider,
) consensus.Consensus {
	dag := model.NewDag()
	layering := f.LayeringFactory.NewLayering(dag, f.Committee)
	return newActiveDagConsensus(dag, layering, server, creator, source, f.EmitInterval)
}

// NewPassive creates a new passive DAG consensus instance parametrized by the factory configuration.
// It does not create/emit events but listens for them on the network in order to reproduce the DAG.
// The reproduced DAG is used to linearize events and their respective transactions, bundling
// and delivering them to any registered listeners.
// The provided server is used for network communication.
func (f Factory) NewPassive(server p2p.Server) consensus.Consensus {
	dag := model.NewDag()
	layering := f.LayeringFactory.NewLayering(dag, f.Committee)
	return newPassiveDagConsensus(dag, layering, server)
}

// Consensus is responsible for coordinating the consensus process, broadcasting new
// events, handling incoming event messages from peers, maintaining the DAG,
// and linearizing the events based on the assigned [layering.Layering] algorithm.
type Consensus struct {
	creator  consensus.ValidatorId
	dag      *model.Dag
	layering layering.Layering

	// leaderCandidates stores all current candidates for leader election which
	// are given the [layering.VerdictUndecided] verdict within the current DAG.
	leaderCandidates []*model.Event
	// deliveredEvents keeps track of all events that have already been delivered
	// to bundle listeners, in order to avoid double-delivery.
	deliveredEvents  sets.Set[*model.Event]
	nextBundleNumber uint32

	seenEvents sets.Set[model.EventId]
	emitter    *emitter.Emitter[model.EventMessage]
	channel    broadcast.Channel[model.EventMessage]
	// receiver is needed for unregistering from the gossip on [Consensus.Stop].
	receiver broadcast.Receiver[model.EventMessage]

	listeners []consensus.BundleListener

	eventProcessingMutex sync.Mutex
	seenEventsMutex      sync.Mutex
}

func newActiveDagConsensus(
	dag *model.Dag,
	layering layering.Layering,
	server p2p.Server,
	creator consensus.ValidatorId,
	transactionProvider consensus.TransactionProvider,
	emitInterval time.Duration,
) *Consensus {
	consensus := newPassiveDagConsensus(dag, layering, server)
	consensus.creator = creator
	consensus.emitter = emitter.StartSimpleEmitter(
		&emissionPayloadSourceAdapter{consensus: consensus, transactionSource: transactionProvider},
		consensus.channel,
		emitInterval,
	)

	return consensus
}

func newPassiveDagConsensus(dag *model.Dag, layering layering.Layering, server p2p.Server) *Consensus {
	consensus := &Consensus{
		layering:        layering,
		dag:             dag,
		seenEvents:      sets.Empty[model.EventId](),
		deliveredEvents: sets.Empty[*model.Event](),
	}
	consensus.channel = broadcast.NewGossip(
		server,
		func(msg model.EventMessage) model.EventId { return msg.EventId() },
	)
	consensus.receiver = broadcast.WrapReceiver(func(message model.EventMessage) {
		consensus.processEventMessage(message)
	})
	consensus.channel.Register(consensus.receiver)

	return consensus
}

// RegisterListener registers a new bundle listener to receive notifications
// about new bundles emitted by the local DAG consensus instance.
func (c *Consensus) RegisterListener(listener consensus.BundleListener) {
	if listener != nil {
		c.eventProcessingMutex.Lock()
		defer c.eventProcessingMutex.Unlock()
		c.listeners = append(c.listeners, listener)
	}
}

// Stop stops the instance's event emission, event processing and bundle
// production. It blocks until the emission loop exits.
func (c *Consensus) Stop() {
	c.channel.Unregister(c.receiver)

	if c.emitter != nil {
		c.emitter.Stop()
		c.emitter = nil
	}
}

func (c *Consensus) processEventMessage(msg model.EventMessage) {
	c.seenEventsMutex.Lock()
	if c.seenEvents.Contains(msg.EventId()) {
		c.seenEventsMutex.Unlock()
		return
	}
	c.seenEvents.Add(msg.EventId())
	c.seenEventsMutex.Unlock()

	// DAG processing is outside of the main processing mutex as DAG can be
	// updated in parallel with candidate/leader processing.
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
			isLeader := c.layering.IsLeader(candidate)
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

	// The closure subtraction between consecutive leaders should be strictly monotonic
	// i.e. newLeader.GetClosure() should be a strict superset of prevLeader.GetClosure().
	// To this end, we sort/filter the leaders based on the associated Layering policy,
	// provided with the current DAG context.
	newLeaders = c.layering.SortLeaders(newLeaders)
	// Deliver respective delta closures of each leader, in a deterministic manner.
	for _, leader := range newLeaders {
		newCovered := sets.Empty[*model.Event]()

		leader.TraverseClosure(
			model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
				if c.deliveredEvents.Contains(e) {
					return model.Visit_Prune
				}
				newCovered.Add(e)
				return model.Visit_Descent
			}))

		// TODO: make layering contribute to sorting or find a better universal way.
		sortedClosure := slices.SortedFunc(newCovered.All(), func(a, b *model.Event) int {
			return bytes.Compare(a.EventId().Serialize(), b.EventId().Serialize())
		})
		c.deliverConfirmedEvents(sortedClosure)

		c.deliveredEvents.AddAll(newCovered)
	}
}

// deliverConfirmedEvents bundles transactions from events, keeping their respective
// order, delivering them to registered bundle listeners.
// The caller should hold the eventProcessingMutex as the method competes for resources
// with other parts of the code.
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

	return eventMessage
}

// emissionPayloadSourceAdapter implements the [emitter.EmissionPayloadSource] interface.
// This adapter makes emitter integration private, i.e. relieves the Consensus
// of the responsibility of implementing this interface directly.
type emissionPayloadSourceAdapter struct {
	consensus         *Consensus
	transactionSource consensus.TransactionProvider
}

func (c *emissionPayloadSourceAdapter) GetEmissionPayload() model.EventMessage {
	return c.consensus.createNewEvent(c.transactionSource.GetCandidateTransactions())
}
