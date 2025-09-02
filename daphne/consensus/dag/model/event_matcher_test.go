package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventMatcher_WithEventId_MatchesWithAnotherEventId(t *testing.T) {
	id := EventId{1}
	matcher := WithEventId(id)

	require.True(t, matcher.Matches(&Event{id: id}))
}

func TestEventMatcher_WithEventId_DoesNotMatchWithAnotherEventId(t *testing.T) {
	matcher := WithEventId(EventId{1})

	require.False(t, matcher.Matches(&Event{id: EventId{2}}))
}

func TestEventMatcher_WithEventId_DoesNotMatchWithAnotherNonEvent(t *testing.T) {
	matcher := WithEventId(EventId{1})

	require.False(t, matcher.Matches(struct{}{}))
}

func TestEventMatcher_String_PrintsMessageStringPayload(t *testing.T) {
	matcher := WithEventId(EventId{1, 15, 16})

	require.Equal(t, "Event with ID: is equal to 010F100000000000 (model.EventId)", matcher.String())
}
