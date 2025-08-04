package model

import (
	"maps"
	"slices"
	"sync"
)

type Dag struct {
	store   *Store
	pending []EventMessage

	tips   map[CreatorId]*Event
	tipsMu *sync.Mutex
}

func NewDag() Dag {
	return Dag{
		store:   &Store{},
		pending: []EventMessage{},
		tips:    make(map[CreatorId]*Event),
		tipsMu:  &sync.Mutex{},
	}
}

func (d *Dag) AddEvent(eventMessage EventMessage) []*Event {
	if slices.ContainsFunc(d.pending, func(e EventMessage) bool {
		return e.EventId() == eventMessage.EventId()
	}) {
		return nil
	}

	d.pending = append(d.pending, eventMessage)

	connected := make([]*Event, 0)
	for {
		newFound := false
		d.pending = slices.DeleteFunc(d.pending, func(e EventMessage) bool {
			if event, isConnected := d.tryConnectEvent(e); isConnected {

				connected = append(connected, event)
				newFound = true
				if d.store == nil {
					d.store = &Store{}
				}
				d.store.add(event)
				return true
			}
			return false
		})
		if !newFound {
			break
		}
	}

	// Track tips
	if d.tipsMu == nil {
		d.tipsMu = &sync.Mutex{}
	}
	d.tipsMu.Lock()
	defer d.tipsMu.Unlock()
	for _, event := range connected {
		mostRecent, exists := d.tips[event.Creator]
		if !exists {
			d.tips[event.Creator] = event
			continue
		}
		seqMostRecent := d.GetSequenceNumber(mostRecent)
		seqNew := d.GetSequenceNumber(event)
		if seqNew > seqMostRecent {
			d.tips[event.Creator] = event
		}
	}
	return connected
}

func (d *Dag) GetClosure(event *Event) []*Event {
	if event == nil {
		return []*Event{}
	}
	closure := []*Event{event}
	for _, parent := range event.Parents {
		closure = append(closure, d.GetClosure(parent)...)
	}
	return closure
}

func (d *Dag) GetTips() map[CreatorId]*Event {
	if d.tipsMu == nil {
		d.tipsMu = &sync.Mutex{}
	}
	d.tipsMu.Lock()
	defer d.tipsMu.Unlock()
	return maps.Clone(d.tips)
}

func (d *Dag) GetSequenceNumber(event *Event) int64 {
	return int64(event.Seq)
}

func (d *Dag) tryConnectEvent(eventMessage EventMessage) (*Event, bool) {
	parentEvents := make([]*Event, 0, len(eventMessage.Parents))
	for _, parent := range eventMessage.Parents {
		if parentEvent, exists := d.store.Get(parent); exists {
			parentEvents = append(parentEvents, parentEvent)
		} else {
			return nil, false
		}
	}
	event := &Event{
		Creator: eventMessage.Creator,
		Parents: parentEvents,
		Payload: eventMessage.Payload,
	}
	if event.SelfParent() == nil {
		event.Seq = 1
	} else {
		event.Seq = event.SelfParent().Seq + 1
	}
	return event, true
}
