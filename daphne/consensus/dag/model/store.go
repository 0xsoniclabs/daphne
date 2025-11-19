package model

import (
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
)

// store is a simple in-memory store for events.
// It provides thread-safe access to events by their EventId.
type store[P payload.Payload] struct {
	data  map[EventId]*Event[P]
	mutex sync.Mutex
}

// get retrieves an event by its EventId from the store.
func (s *store[P]) get(eventId EventId) (*Event[P], bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	event, exists := s.data[eventId]
	return event, exists
}

// add adds an event to the store.
func (s *store[P]) add(event *Event[P]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.data == nil {
		s.data = make(map[EventId]*Event[P])
	}
	s.data[event.EventId()] = event
}
