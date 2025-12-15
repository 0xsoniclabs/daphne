package dag

import (
	"bytes"
	"fmt"
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/emitter"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

// Factory defines the configuration for the DAG consensus algorithm instance:
//   - EmitInterval: the interval at which events are created and gossiped (Active instance only).
//   - Creator: the ID of the creator of the events (Active instance only).
//   - Committee: the creator committee which participates in DAG building and [layering.Layering].
//   - LayeringFactory: the factory configuration used to instantiate the layering algorithm.
type Factory[P payload.Payload] struct {
	EmitterFactory         emitter.Factory
	LayeringFactory        layering.Factory
	PayloadProtocolFactory payload.ProtocolFactory[P]
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
	dag := model.NewDag(&committee)
	layering := f.LayeringFactory.NewLayering(dag, &committee)
	payload := f.PayloadProtocolFactory.NewProtocol(&committee, creator)
	return newActiveDagConsensus(dag, layering, payload, server, creator, source, f.EmitterFactory)
}

// NewPassive creates a new passive DAG consensus instance parametrized by the factory configuration.
// It does not create/emit events but listens for them on the network in order to reproduce the DAG.
// The reproduced DAG is used to linearize events and their respective transactions, bundling
// and delivering them to any registered listeners.
// The provided server is used for network communication.
func (f Factory[P]) NewPassive(server p2p.Server, committee consensus.Committee) consensus.Consensus {
	dag := model.NewDag(&committee)
	layering := f.LayeringFactory.NewLayering(dag, &committee)
	payload := f.PayloadProtocolFactory.NewProtocol(&committee, 0)
	return newPassiveDagConsensus(dag, layering, payload, server)
}

// String returns a human-readable summary of the factory configuration.
func (f Factory[P]) String() string {
	return fmt.Sprintf(
		"%s-%s-%s",
		f.LayeringFactory.String(),
		f.PayloadProtocolFactory.String(),
		f.EmitterFactory.String(),
	)
}

// Consensus is responsible for coordinating the consensus process, broadcasting new
// events, handling incoming event messages from peers, maintaining the DAG,
// and linearizing the events based on the assigned [layering.Layering] algorithm.
type Consensus[P payload.Payload] struct {
	creator  consensus.ValidatorId
	dag      model.Dag
	layering layering.Layering
	payloads payload.Protocol[P]

	// leaderCandidates stores all current candidates for leader election which
	// are given the [layering.VerdictUndecided] verdict within the current DAG.
	leaderCandidates []*model.Event
	// deliveredEvents keeps track of all events that have already been delivered
	// to bundle listeners, in order to avoid double-delivery.
	deliveredEvents  sets.Set[*model.Event]
	nextBundleNumber uint32

	seenEvents sets.Set[model.EventId]
	emitter    emitter.Emitter
	channel    broadcast.Channel[EventMessage[P]]
	// receiver is needed for unregistering from the gossip on [Consensus.Stop].
	receiver broadcast.Receiver[EventMessage[P]]

	listeners *consensus.BundleListenerManager

	eventProcessingMutex sync.Mutex
	seenEventsMutex      sync.Mutex
	emitterMutex         sync.Mutex
}

func newActiveDagConsensus[P payload.Payload](
	dag model.Dag,
	layering layering.Layering,
	payloads payload.Protocol[P],
	server p2p.Server,
	creator consensus.ValidatorId,
	transactionProvider consensus.TransactionProvider,
	emitterFactory emitter.Factory,
) *Consensus[P] {
	consensus := newPassiveDagConsensus(dag, layering, payloads, server)
	consensus.creator = creator
	consensus.emitterMutex.Lock()
	consensus.emitter = emitterFactory.NewEmitter(
		wrapEmitChannel(consensus, transactionProvider),
		dag,
		creator,
	)
	consensus.emitterMutex.Unlock()
	consensus.emitter.OnChange() // Initial emission
	return consensus
}

func newPassiveDagConsensus[P payload.Payload](
	dag model.Dag,
	layering layering.Layering,
	payloads payload.Protocol[P],
	server p2p.Server,
) *Consensus[P] {
	consensus := &Consensus[P]{
		listeners:       consensus.NewBundleListenerManager(),
		layering:        layering,
		payloads:        payloads,
		dag:             dag,
		seenEvents:      sets.Empty[model.EventId](),
		deliveredEvents: sets.Empty[*model.Event](),
	}
	consensus.channel = broadcast.NewGossip(
		server,
		func(msg EventMessage[P]) model.EventId { return msg.EventId() },
	)
	consensus.receiver = broadcast.WrapReceiver(consensus.processEventMessage)
	consensus.channel.Register(consensus.receiver)

	return consensus
}

// RegisterListener registers a new bundle listener to receive notifications
// about new bundles emitted by the local DAG consensus instance.
func (c *Consensus[P]) RegisterListener(listener consensus.BundleListener) {
	c.listeners.RegisterListener(listener)
}

// Stop stops the instance's event emission, event processing and bundle
// production. It blocks until the emission loop exits.
func (c *Consensus[P]) Stop() {
	// The emitter is stopped first to prevent spawning new
	// delivery threads by the receiver while unregistering from the channel.
	if c.emitter != nil {
		c.emitter.Stop()
	}

	c.channel.Unregister(c.receiver)

	if c.listeners != nil {
		c.listeners.Stop()
		c.listeners = nil
	}
}

func (c *Consensus[P]) processEventMessage(msg EventMessage[P]) {
	c.seenEventsMutex.Lock()
	if c.seenEvents.Contains(msg.EventId()) {
		c.seenEventsMutex.Unlock()
		return
	}
	c.seenEvents.Add(msg.EventId())
	c.seenEventsMutex.Unlock()

	// DAG processing is outside of the main processing mutex as DAG can be
	// updated in parallel with candidate/leader processing.
	connected := c.dag.AddEvent(msg.raw())

	// TODO: I don't like this mutex, but there is a race condition being repored without it.
	c.emitterMutex.Lock()
	if c.emitter != nil && len(connected) > 0 {
		go c.emitter.OnChange()
	}
	c.emitterMutex.Unlock()

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
func (c *Consensus[P]) deliverConfirmedEvents(events []*model.Event) {
	payloads := make([]P, len(events))
	for i, event := range events {
		payload := event.Payload()
		if payload == nil {
			continue
		}
		payloads[i] = payload.(P)
	}

	for _, bundle := range c.payloads.Merge(payloads) {
		c.nextBundleNumber++
		bundle.Number = c.nextBundleNumber
		c.listeners.NotifyListeners(bundle)
	}
}

func (c *Consensus[P]) createNewEvent(dagHeads map[consensus.ValidatorId]*model.Event, lineup txpool.Lineup) EventMessage[P] {
	parents := []*model.Event{}
	if _, found := dagHeads[c.creator]; found {
		parents = append(parents, dagHeads[c.creator])
		for creator, tip := range dagHeads {
			if creator != c.creator {
				parents = append(parents, tip)
			}
		}
	}

	// Obtain the maximum round of the parents.
	maxRoundOfParents := uint32(0)
	c.eventProcessingMutex.Lock() // < need due to layering access
	for _, parent := range parents {
		maxRoundOfParents = max(maxRoundOfParents, c.layering.GetRound(parent))
	}
	c.eventProcessingMutex.Unlock()

	// Retrieve the payload for the new event.
	payload := c.payloads.BuildPayload(
		payload.EventMeta{
			ParentsMaxRound: maxRoundOfParents,
		},
		lineup,
	)

	// Build event message
	parentIds := []model.EventId{}
	for _, parent := range parents {
		parentIds = append(parentIds, parent.EventId())
	}
	return makeEventMessage(
		c.creator,
		parentIds,
		payload,
	)
}

// EventMessage is a wrapper around model.EventMessage that provides
// type-safe access to the payload.
type EventMessage[P payload.Payload] struct {
	nested model.EventMessage
}

func (em EventMessage[P]) EventId() model.EventId {
	return em.nested.EventId()
}

func (em EventMessage[P]) raw() model.EventMessage {
	return em.nested
}

func makeEventMessage[P payload.Payload](
	creator consensus.ValidatorId,
	parents []model.EventId,
	payload P,
) EventMessage[P] {
	return EventMessage[P]{
		nested: model.EventMessage{
			Creator: creator,
			Parents: parents,
			Payload: payload,
		},
	}
}

// wrapEmitChannel wraps the consensus instance and a transaction source
// into an [emitter.Channel] in an effort to decouple the payload and
// network logic from the emission logic.
func wrapEmitChannel[P payload.Payload](
	consenus *Consensus[P],
	transactionProvider consensus.TransactionProvider,
) emitter.Channel {
	return &emitChannel[P]{
		Consensus:           consenus,
		transactionProvider: transactionProvider,
	}
}

type emitChannel[P payload.Payload] struct {
	*Consensus[P]
	transactionProvider consensus.TransactionProvider
}

func (e *emitChannel[P]) Emit(dagHeads map[consensus.ValidatorId]*model.Event) {
	e.channel.Broadcast(
		e.createNewEvent(dagHeads, e.transactionProvider.GetCandidateLineup()),
	)
}
