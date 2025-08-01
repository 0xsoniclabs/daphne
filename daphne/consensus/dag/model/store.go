package model

import "sync"

type Store struct {
	data  map[EventId]Event
	mutex sync.Mutex
}

func (s *Store) Get(eventId EventId) (Event, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	event, exists := s.data[eventId]
	return event, exists
}

func (s *Store) add(event Event) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.data == nil {
		s.data = make(map[EventId]Event)
	}
	s.data[event.EventId()] = event
}
