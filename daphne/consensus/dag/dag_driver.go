package dag

import (
	"bytes"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
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
type Factory[P payload.Payload] struct {
	EmitInterval    time.Duration
	LayeringFactory layering.Factory[P]
	PayloadProtocol payload.Protocol[P]
}

// NewActive creates a new active DAG consensus instance parametrized by the factory configuration.
// The active instance produced by NewActive extends the responsibility of the instance
// created by [Factory.NewPassive], by creating and periodically emitting DAG events.
// The source is used to get candidate transactions for event emission, and the provided
// server is used for P2P communication.
func (f Factory[P]) NewActive(
	server p2p.Server,
	committee consensus.Committee,
	creator consensus.ValidatorId,
	source consensus.TransactionProvider,
) consensus.Consensus {
	dag := model.NewDag[P]()
	layering := f.LayeringFactory.NewLayering(dag, &committee)
	return newActiveDagConsensus(dag, layering, f.PayloadProtocol, server, creator, source, f.EmitInterval)
}

// NewPassive creates a new passive DAG consensus instance parametrized by the factory configuration.
// It does not create/emit events but listens for them on the network in order to reproduce the DAG.
// The reproduced DAG is used to linearize events and their respective transactions, bundling
// and delivering them to any registered listeners.
// The provided server is used for network communication.
func (f Factory[P]) NewPassive(server p2p.Server, committee consensus.Committee) consensus.Consensus {
	dag := model.NewDag[P]()
	layering := f.LayeringFactory.NewLayering(dag, &committee)
	return newPassiveDagConsensus(dag, layering, f.PayloadProtocol, server)
}

// String returns a human-readable summary of the factory configuration.
func (f Factory[P]) String() string {
	return fmt.Sprintf(
		"%s-%s-%dms",
		f.LayeringFactory.String(),
		f.PayloadProtocol.String(),
		f.EmitInterval.Milliseconds(),
	)
}

// Consensus is responsible for coordinating the consensus process, broadcasting new
// events, handling incoming event messages from peers, maintaining the DAG,
// and linearizing the events based on the assigned [layering.Layering] algorithm.
type Consensus[P payload.Payload] struct {
	creator  consensus.ValidatorId
	dag      model.Dag[P]
	layering layering.Layering[P]
	payloads payload.Protocol[P]

	// leaderCandidates stores all current candidates for leader election which
	// are given the [layering.VerdictUndecided] verdict within the current DAG.
	leaderCandidates []*model.Event[P]
	// deliveredEvents keeps track of all events that have already been delivered
	// to bundle listeners, in order to avoid double-delivery.
	deliveredEvents  sets.Set[*model.Event[P]]
	nextBundleNumber uint32

	seenEvents sets.Set[model.EventId]
	emitter    *concurrent.Job
	channel    broadcast.Channel[model.EventMessage[P]]
	// receiver is needed for unregistering from the gossip on [Consensus.Stop].
	receiver broadcast.Receiver[model.EventMessage[P]]

	listeners []consensus.BundleListener

	eventProcessingMutex sync.Mutex
	seenEventsMutex      sync.Mutex
}

func newActiveDagConsensus[P payload.Payload](
	dag model.Dag[P],
	layering layering.Layering[P],
	payloads payload.Protocol[P],
	server p2p.Server,
	creator consensus.ValidatorId,
	transactionProvider consensus.TransactionProvider,
	emitInterval time.Duration,
) *Consensus[P] {
	consensus := newPassiveDagConsensus(dag, layering, payloads, server)
	consensus.creator = creator
	consensus.emitter = concurrent.StartPeriodicJob(
		emitInterval,
		func(time.Time) {
			candidates := transactionProvider.GetCandidateTransactions()
			event := consensus.createNewEvent(candidates)
			consensus.channel.Broadcast(event)
		},
	)
	return consensus
}

func newPassiveDagConsensus[P payload.Payload](
	dag model.Dag[P],
	layering layering.Layering[P],
	payloads payload.Protocol[P],
	server p2p.Server,
) *Consensus[P] {
	consensus := &Consensus[P]{
		layering:        layering,
		payloads:        payloads,
		dag:             dag,
		seenEvents:      sets.Empty[model.EventId](),
		deliveredEvents: sets.Empty[*model.Event[P]](),
	}
	consensus.channel = broadcast.NewGossip(
		server,
		func(msg model.EventMessage[P]) model.EventId { return msg.EventId() },
	)
	consensus.receiver = broadcast.WrapReceiver(func(message model.EventMessage[P]) {
		consensus.processEventMessage(message)
	})
	consensus.channel.Register(consensus.receiver)

	return consensus
}

// RegisterListener registers a new bundle listener to receive notifications
// about new bundles emitted by the local DAG consensus instance.
func (c *Consensus[P]) RegisterListener(listener consensus.BundleListener) {
	if listener != nil {
		c.eventProcessingMutex.Lock()
		defer c.eventProcessingMutex.Unlock()
		c.listeners = append(c.listeners, listener)
	}
}

// Stop stops the instance's event emission, event processing and bundle
// production. It blocks until the emission loop exits.
func (c *Consensus[P]) Stop() {
	c.channel.Unregister(c.receiver)

	if c.emitter != nil {
		c.emitter.Stop()
		c.emitter = nil
	}
}

func (c *Consensus[P]) processEventMessage(msg model.EventMessage[P]) {
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

	newLeaders := []*model.Event[P]{}
	// Look for new leaders and remove events that lost the candidate status.
	c.leaderCandidates = slices.DeleteFunc(
		c.leaderCandidates,
		func(candidate *model.Event[P]) bool {
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
		newCovered := sets.Empty[*model.Event[P]]()

		leader.TraverseClosure(
			model.WrapEventVisitor(func(e *model.Event[P]) model.VisitResult {
				if c.deliveredEvents.Contains(e) {
					return model.Visit_Prune
				}
				newCovered.Add(e)
				return model.Visit_Descent
			}))

		// TODO: make layering contribute to sorting or find a better universal way.
		sortedClosure := slices.SortedFunc(newCovered.All(), func(a, b *model.Event[P]) int {
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
func (c *Consensus[P]) deliverConfirmedEvents(events []*model.Event[P]) {
	payloads := make([]P, len(events))
	for i, event := range events {
		payloads[i] = event.Payload()
	}

	for _, bundle := range c.payloads.Merge(payloads) {
		c.nextBundleNumber++
		bundle.Number = c.nextBundleNumber
		for _, listener := range c.listeners {
			listener.OnNewBundle(bundle)
		}
	}
}

func (c *Consensus[P]) createNewEvent(candidates []types.Transaction) model.EventMessage[P] {
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
	eventMessage := model.EventMessage[P]{
		Creator: c.creator,
		Parents: parents,
		Payload: c.payloads.BuildPayload(candidates),
	}

	return eventMessage
}
