package model

import (
	"fmt"
	"maps"
	"slices"
)

type Dag struct {
	store   *Store
	pending []Event

	tips map[CreatorId]EventId
}

func (d *Dag) AddEvent(event Event) []Event {
	if d.store == nil {
		d.store = &Store{}
	}
	d.store.add(event)

	known := slices.ContainsFunc(d.pending, func(e Event) bool {
		return e.EventId() == event.EventId()
	})
	if known {
		return nil
	}
	d.pending = append(d.pending, event)

	connected := []Event{}
	for {
		newFound := false
		d.pending = slices.DeleteFunc(d.pending, func(e Event) bool {
			if d.isConnected(e) {
				connected = append(connected, e)
				newFound = true
				return true
			}
			return false
		})
		if !newFound {
			break
		}
	}

	// Track tips
	if d.tips == nil {
		d.tips = make(map[CreatorId]EventId)
	}

	for _, event := range connected {
		mostRecent, exists := d.tips[event.Creator]
		if !exists {
			d.tips[event.Creator] = event.EventId()
			continue
		}
		seqMostRecent, err := d.GetSequenceNumber(mostRecent)
		if err != nil {
			panic(fmt.Sprintf("failed to get sequence number for most recent event: %s, error: %v", mostRecent, err))
		}
		seqNew, err := d.GetSequenceNumber(event.EventId())
		if err != nil {
			panic(fmt.Sprintf("failed to get sequence number for new event: %s, error: %v", event.EventId(), err))
		}
		if seqNew > seqMostRecent {
			d.tips[event.Creator] = event.EventId()
		}
	}

	return connected
}

func (d *Dag) GetClosure(eventId EventId) []Event {
	panic("not implemented")
}

func (d *Dag) GetTips() map[CreatorId]EventId {
	return maps.Clone(d.tips)
}

func (d *Dag) GetSequenceNumber(id EventId) (int64, error) {
	event, found := d.store.Get(id)
	if !found {
		return 0, fmt.Errorf("event not found: %s", id)
	}
	selfParent := event.SelfParent()
	if selfParent == nil {
		return 0, nil
	}
	return d.GetSequenceNumber(*selfParent)
}

func (d *Dag) isConnected(event Event) bool {
	for _, parent := range event.Parents {
		if _, exists := d.store.Get(parent); !exists {
			return false
		}
	}
	return true
}
