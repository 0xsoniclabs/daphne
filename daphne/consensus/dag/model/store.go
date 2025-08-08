package model

import "sync"

// store is a simple in-memory store for events.
// It provides thread-safe access to events by their EventId.
type store struct {
	data  map[EventId]*Event
	mutex sync.Mutex
}

// get retrieves an event by its EventId from the store.
func (s *store) get(eventId EventId) (*Event, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	event, exists := s.data[eventId]
	return event, exists
}

// add adds an event to the store.
func (s *store) add(event *Event) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.data == nil {
		s.data = make(map[EventId]*Event)
	}
	s.data[event.EventId()] = event
}
