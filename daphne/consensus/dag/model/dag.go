package model

import (
	"maps"
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
)

// Dag represents a Directed Acyclic Graph (DAG) structure for managing events.
type Dag struct {
	// store is a thread-safe mapping of event IDs to Event objects.
	store *store

	// pending is a slice of EventMessages that are pending to be added to the DAG.
	// They are pending until all their parents are present in the DAG.
	pending []EventMessage
	// pendingMu is a mutex to protect access to the pending slice.
	pendingMu *sync.Mutex

	// heads is a map of CreatorId to the most recent Event for that creator.
	heads map[consensus.ValidatorId]*Event
	// headsMu is a mutex to protect access to the heads map.
	headsMu *sync.Mutex

	vecMutex         sync.Mutex
	lowestAfter      map[*Event][]*Event
	highestBefore    map[*Event][]*Event
	validatorIdToIdx map[consensus.ValidatorId]int
	validatorIdxToId map[int]consensus.ValidatorId
	committee        *consensus.Committee
}

// NewDag initializes a new, empty Dag.
func NewDag(committee *consensus.Committee) *Dag {
	validatorIdToIdx := make(map[consensus.ValidatorId]int, len(committee.Creators()))
	validatorIdxToId := make(map[int]consensus.ValidatorId, len(committee.Creators()))
	for i, creatorId := range committee.Creators() {
		validatorIdToIdx[creatorId] = i
		validatorIdxToId[i] = creatorId
	}
	return &Dag{
		store:            &store{},
		pending:          []EventMessage{},
		pendingMu:        &sync.Mutex{},
		heads:            make(map[consensus.ValidatorId]*Event),
		headsMu:          &sync.Mutex{},
		lowestAfter:      make(map[*Event][]*Event),
		highestBefore:    make(map[*Event][]*Event),
		committee:        committee,
		validatorIdToIdx: validatorIdToIdx,
		validatorIdxToId: validatorIdxToId,
	}
}

// AddEvent adds an event to the DAG and connects it to its parents
// if they are already present in the DAG. If a parent is not yet present, the
// event is kept in a temporary buffer, and re-evaluated as future events are
// added. The function returns a list of all events that got connected to the
// DAG through the addition of the given node.
func (d *Dag) AddEvent(eventMessage EventMessage) []*Event {
	// Check if the event is already present in the store.
	if _, exists := d.store.get(eventMessage.EventId()); exists {
		return nil // Event already exists, no need to add it again.
	}

	connected := d.updatePending(eventMessage)

	for _, event := range connected {
		d.setLowestAfter(event)
		d.setHighestBefore(event)
	}

	// Track heads.
	d.headsMu.Lock()
	defer d.headsMu.Unlock()
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

// GetHeads returns a copy of the current heads of the DAG, which are the most recent events
// for each creator. It is from among the heads that the parents for new events are selected.
// Adding a non-head event to an Event's parents list is not allowed.
func (d *Dag) GetHeads() map[consensus.ValidatorId]*Event {
	d.headsMu.Lock()
	defer d.headsMu.Unlock()
	return maps.Clone(d.heads)
}

// tryConnectEvent attempts to connect an event to its parents.
// If all parents are present in the DAG, it creates a new Event
// from an EventMessage and returns it. If any parent is missing,
// it returns nil and false.
func (d *Dag) tryConnectEvent(eventMessage EventMessage) (*Event, bool) {
	parentEvents := make([]*Event, 0, len(eventMessage.Parents))
	for _, parent := range eventMessage.Parents {
		parentEvent, exists := d.store.get(parent)
		if !exists {
			return nil, false
		}
		parentEvents = append(parentEvents, parentEvent)
	}
	event, err := NewEvent(eventMessage.Creator, parentEvents, eventMessage.Payload)
	if err != nil {
		panic("TODO: Dag is currently not equipped to handle erroneous messages")
	}
	return event, true
}

// updatePending adds an event message to the pending list and attempts to connect it
// to its parents. If the event can be connected, it is added to the DAG and
// returned. If the event is already pending, it is ignored.
// The function returns a slice of connected events.
func (d *Dag) updatePending(eventMessage EventMessage) []*Event {
	d.pendingMu.Lock()
	defer d.pendingMu.Unlock()

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

func (d *Dag) setLowestAfter(event *Event) {
	vec := make([]*Event, len(d.committee.Creators()))

	d.vecMutex.Lock()
	defer d.vecMutex.Unlock()

	myIdx := d.validatorIdToIdx[event.Creator()]
	d.lowestAfter[event] = vec
	// Set yourself as your own lowest after.

	var traverse func(*Event)
	traverse = func(currentEvent *Event) {
		currentLowestAfter := d.lowestAfter[currentEvent]
		if currentLowestAfter[myIdx] != nil {
			// If you have already observed one of your own events, stop traversing.
			return
		}
		// Otherwise, update other event's lowest after
		currentLowestAfter[myIdx] = event
		for _, parent := range currentEvent.Parents() {
			traverse(parent)
		}
	}
	traverse(event)

}

func (d *Dag) GetLowestAfter(event *Event) []*Event {
	if vec, ok := d.lowestAfter[event]; ok {
		return slices.Clone(vec)
	}
	return make([]*Event, len(d.committee.Creators()))
}

func (d *Dag) setHighestBefore(event *Event) {
	vec := make([]*Event, len(d.committee.Creators()))

	d.vecMutex.Lock()
	defer d.vecMutex.Unlock()

	// Set yourself as your own highest before.
	vec[d.validatorIdToIdx[event.Creator()]] = event

	for idx := range vec {
		seq := uint32(0)
		if vec[idx] != nil {
			seq = vec[idx].Seq()
		}
		for _, parent := range event.Parents() {
			parentHighestBefore := d.GetHighestBefore(parent)

			if parentHighestBefore[idx] != nil && parentHighestBefore[idx].Seq() > seq {
				vec[idx] = parentHighestBefore[idx]
			}
		}
	}

	d.highestBefore[event] = vec
}

func (d *Dag) GetHighestBefore(event *Event) []*Event {
	if vec, ok := d.highestBefore[event]; ok {
		return slices.Clone(vec)
	}
	return nil
}

func (d *Dag) StronglyReaches(a, b *Event) bool {
	d.vecMutex.Lock()
	highestBefore := d.GetHighestBefore(a)
	lowestAfter := d.GetLowestAfter(b)
	d.vecMutex.Unlock()

	if lowestAfter == nil || highestBefore == nil {
		return false
	}
	voteCounter := consensus.NewVoteCounter(d.committee)

	for i := range lowestAfter {
		if lowestAfter[i] == nil || highestBefore[i] == nil {
			continue
		}
		if highestBefore[i].Seq() >= lowestAfter[i].Seq() {
			voteCounter.Vote(d.validatorIdxToId[i])
		}
	}

	return voteCounter.IsQuorumReached()
}

func (d *Dag) Reaches(a, b *Event) bool {
	highestBefore := d.GetHighestBefore(a)
	lowestAfter := d.GetLowestAfter(b)

	if lowestAfter == nil || highestBefore == nil {
		return false
	}

	for i := range lowestAfter {
		if lowestAfter[i] == nil || highestBefore[i] == nil {
			continue
		}
		if highestBefore[i].Seq() >= lowestAfter[i].Seq() {
			return true
		}
	}
	return false
}
