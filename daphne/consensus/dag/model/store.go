// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

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
