package model

import (
	"fmt"

	"go.uber.org/mock/gomock"
)

// WithEventId is a gomock matcher for [*Event] and [EventMessage] instances that
// checks whether a given instance has a specified id. The id can be any type that
// implements the gomock.Matcher interface, or a specific value that should be
// matched exactly.
func WithEventId(id any) gomock.Matcher {
	if matcher, ok := id.(gomock.Matcher); ok {
		return withEventId{id: matcher}
	}
	return WithEventId(gomock.Eq(id))
}

type withEventId struct {
	id gomock.Matcher
}

func (i withEventId) Matches(arg any) bool {
	event, ok := arg.(*Event)
	if !ok {
		return false
	}
	return i.id.Matches(event.EventId())
}

func (i withEventId) String() string {
	return fmt.Sprintf("Event with ID: %s", i.id.String())
}
