package model

import (
	"maps"
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
)

//go:generate mockgen -source dag.go -destination=dag_mock.go -package=model

// Dag represents a Directed Acyclic Graph (DAG) structure for managing events.
type Dag interface {
	// AddEvent adds an event to the DAG and connects it to its parents
	// if they are already present in the DAG. If a parent is not yet present, the
	// event is buffered, and re-evaluated as future events are added.
	// The function returns a list of all events that got connected to the
	// DAG through the addition of the given node.
	AddEvent(eventMessage EventMessage) []*Event
	// GetHeads returns a mapping of each validator to their current head of the DAG,
	// which represent the most recent event for each of the validators.
	GetHeads() map[consensus.ValidatorId]*Event
	// Reaches returns true if the source event reaches the target event,
	// i.e., if there exists a path from source to target in the DAG.
	Reaches(source, target *Event) bool
	// StronglyReaches returns true if the source event strongly reaches target event,
	// i.e., if there exists a quorum of events reachable from source that also reach target.
	StronglyReaches(source, target *Event) bool
}

// dag is an implementation of the Dag interface.
type dag struct {
	// store is a thread-safe mapping of event IDs to Event objects.
	store *store

	// committee represents the set of consensus participants associated with the DAG.
	committee *consensus.Committee
	// pending is a slice of EventMessages that are pending to be added to the DAG.
	// They are pending until all their parents are present in the DAG.
	pending []EventMessage
	// pendingMutex is a mutex to protect access to the pending slice.
	pendingMutex sync.Mutex

	// heads is a map of CreatorId to the most recent Event for that creator.
	heads map[consensus.ValidatorId]*Event
	// headsMutex is a mutex to protect access to the heads map.
	headsMutex sync.Mutex

	// lowestAfter is a mapping from an Event to a vector indicating, for each validator,
	// the lowest sequence number of an event created by that validator that reaches
	// the given event.
	lowestAfter map[*Event][]uint32
	// highestBefore is a mapping from an Event to a vector indicating, for each validator,
	// the highest sequence number of an event created by that validator that is
	// reachable from the given event.
	highestBefore map[*Event][]uint32

	highestBeforeMutex sync.RWMutex
	lowestAfterMutex   sync.RWMutex

	// validatorIdToIdx maps validator IDs to their corresponding indices in the
	// vectors used in highestBefore and lowestAfter.
	validatorIdToIdx map[consensus.ValidatorId]int
	// validatorIdxToId maps indices back to their corresponding validator IDs.
	validatorIdxToId map[int]consensus.ValidatorId
}

func NewDag(committee *consensus.Committee) Dag {
	return newDag(committee)
}

func newDag(committee *consensus.Committee) *dag {
	validatorIdToIdx := make(map[consensus.ValidatorId]int, len(committee.Validators()))
	validatorIdxToId := make(map[int]consensus.ValidatorId, len(committee.Validators()))
	for idx, validatorId := range committee.Validators() {
		validatorIdToIdx[validatorId] = idx
		validatorIdxToId[idx] = validatorId
	}

	return &dag{
		committee:        committee,
		store:            &store{},
		pending:          []EventMessage{},
		heads:            make(map[consensus.ValidatorId]*Event),
		lowestAfter:      make(map[*Event][]uint32),
		highestBefore:    make(map[*Event][]uint32),
		validatorIdToIdx: validatorIdToIdx,
		validatorIdxToId: validatorIdxToId,
	}
}

func (d *dag) AddEvent(eventMessage EventMessage) []*Event {
	// Ignore events from non-committee members.
	if !slices.Contains(d.committee.Validators(), eventMessage.Creator) {
		return nil
	}

	// Check if the event is already present in the store.
	if _, exists := d.store.get(eventMessage.EventId()); exists {
		return nil // Event already exists, no need to add it again.
	}

	connected := d.updatePending(eventMessage)

	// Track heads.
	d.headsMutex.Lock()
	defer d.headsMutex.Unlock()
	for _, event := range connected {
		// If the creator has no head yet, set this event as the head.
		mostRecent, exists := d.heads[event.creator]
		if !exists {
			d.heads[event.creator] = event
			continue
		}
		if event.seq > mostRecent.seq {
			d.heads[event.creator] = event
		}
	}
	return connected
}

func (d *dag) GetHeads() map[consensus.ValidatorId]*Event {
	d.headsMutex.Lock()
	defer d.headsMutex.Unlock()
	return maps.Clone(d.heads)
}

func (d *dag) Reaches(source, target *Event) bool {
	highestBefore := d.getHighestBefore(source)
	lowestAfter := d.getLowestAfter(target)

	if lowestAfter == nil || highestBefore == nil {
		return false
	}

	for i := range lowestAfter {
		if lowestAfter[i] == 0 {
			continue
		}
		// If there exists a validator whose highestBefore for the source event is
		// greater than or equal to its lowestAfter for the target event, it means
		// that there exists at least a single path through which the target is
		// reachable from the source.
		if highestBefore[i] >= lowestAfter[i] {
			return true
		}
	}

	return false
}

func (d *dag) StronglyReaches(source, target *Event) bool {
	highestBefore := d.getHighestBefore(source)
	lowestAfter := d.getLowestAfter(target)

	if lowestAfter == nil || highestBefore == nil {
		return false
	}

	voteCounter := consensus.NewVoteCounter(d.committee)

	for i := range lowestAfter {
		if lowestAfter[i] == 0 {
			continue
		}
		// If the highestBefore slot for a validator is greater than or equal to
		// the lowestAfter slot for the same validator, it means there is an event
		// from that validator that is reachable from source and also reaches the target.
		if highestBefore[i] >= lowestAfter[i] {
			voteCounter.Vote(d.validatorIdxToId[i])
		}
	}

	return voteCounter.IsQuorumReached()
}

// tryConnectEvent attempts to connect an event to its parents.
// If all parents are present in the DAG, it creates a new Event
// from an EventMessage and returns it. If any parent is missing,
// it returns nil and false.
func (d *dag) tryConnectEvent(eventMessage EventMessage) (*Event, bool) {
	parentEvents := make([]*Event, 0, len(eventMessage.Parents))
	for _, parent := range eventMessage.Parents {
		parentEvent, exists := d.store.get(parent)
		if !exists {
			return nil, false
		}
		parentEvents = append(parentEvents, parentEvent)
	}
	event, err := NewEvent(eventMessage.Creator, parentEvents, eventMessage.Payload, eventMessage.Timestamp)
	if err != nil {
		panic("TODO: Dag is currently not equipped to handle erroneous messages")
	}
	return event, true
}

// updatePending adds an event message to the pending list and attempts to connect it
// to its parents. If the event can be connected, it is added to the DAG and
// returned. If the event is already pending, it is ignored.
// The function returns a slice of connected events.
func (d *dag) updatePending(eventMessage EventMessage) []*Event {
	d.pendingMutex.Lock()
	defer d.pendingMutex.Unlock()

	// Checks if the event is already pending.
	if slices.ContainsFunc(d.pending, func(message EventMessage) bool {
		return message.EventId() == eventMessage.EventId()
	}) {
		return nil
	}

	d.pending = append(d.pending, eventMessage)

	// Try to connect each pending event to its parents.
	// Quit the loop when no new connections are made.
	connected := []*Event{}
	for {
		newFound := false
		d.pending = slices.DeleteFunc(d.pending, func(message EventMessage) bool {
			if event, isConnected := d.tryConnectEvent(message); isConnected {
				connected = append(connected, event)
				newFound = true

				d.store.add(event)
				// Update highestBefore and lowestAfter maps once the event is connected.
				// It is convenient to do it here as the updatePending guarantees a topological
				// order of events being connected - if this was done after the updatePending
				// method, there is a chance of children events reaching the update
				// methods before their parents, potentially corrupting the vectors.
				d.updateHighestBefore(event)
				d.updateLowestAfter(event)

				return true
			}
			return false
		})
		if !newFound {
			break
		}
	}
	return connected
}

func (d *dag) updateHighestBefore(event *Event) {
	highestBefore := make([]uint32, len(d.committee.Validators()))
	// Set the event's seq as a highest before for its own creator.
	highestBefore[d.validatorIdToIdx[event.Creator()]] = event.Seq()

	// For each validator, find the highest sequence number among the parents.
	for _, parent := range event.Parents() {
		parentHighestBefore := d.getHighestBefore(parent)

		for validatorIdx := range highestBefore {
			highestBefore[validatorIdx] = max(highestBefore[validatorIdx], parentHighestBefore[validatorIdx])
		}
	}

	d.writeHighestBefore(event, highestBefore)
}

func (d *dag) updateLowestAfter(event *Event) {
	myIdx := d.validatorIdToIdx[event.Creator()]
	// An event is always the lowestAfter event itself for its own creator.
	d.writeLowestAfter(event, myIdx, event.Seq())

	// Traverse the DAG and update lowestAfter slots for event's creator.
	event.TraverseClosure(
		WrapEventVisitor(func(visitedEvent *Event) VisitResult {
			if event == visitedEvent {
				return Visit_Descent
			}

			lowestAfter := d.getLowestAfter(visitedEvent)
			// If the lowestAfter slot for this validator is already set, prune
			// the branch as all of the ancestors are guaranteed to have the
			// lowest possible one.
			if lowestAfter[myIdx] != 0 {
				return Visit_Prune
			}

			d.writeLowestAfter(visitedEvent, myIdx, event.Seq())
			return Visit_Descent
		}))
}

func (d *dag) getHighestBefore(event *Event) []uint32 {
	d.highestBeforeMutex.RLock()
	defer d.highestBeforeMutex.RUnlock()

	if vec, ok := d.highestBefore[event]; ok {
		return vec
	}
	return nil
}

func (d *dag) getLowestAfter(event *Event) []uint32 {
	d.lowestAfterMutex.RLock()
	defer d.lowestAfterMutex.RUnlock()

	if vec, ok := d.lowestAfter[event]; ok {
		return slices.Clone(vec)
	}
	return nil
}

// writeLowestAfter assigns the lowest after event for its validator index for a given event.
// The granularity of updates is per-validator, as lowest after vectors are built incrementally by
// incoming events.
func (d *dag) writeLowestAfter(event *Event, idx int, lowestAfterSeq uint32) {
	d.lowestAfterMutex.Lock()
	defer d.lowestAfterMutex.Unlock()

	if _, ok := d.lowestAfter[event]; !ok {
		d.lowestAfter[event] = make([]uint32, len(d.committee.Validators()))
	}
	d.lowestAfter[event][idx] = lowestAfterSeq
}

// writeHighestBefore writes the highest before vector for a given event.
// The entire vector is written at once, as it is computed in one pass when its
// corresponding event is added.
func (d *dag) writeHighestBefore(event *Event, vec []uint32) {
	d.highestBeforeMutex.Lock()
	defer d.highestBeforeMutex.Unlock()

	d.highestBefore[event] = vec
}
