package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvent_EventId_EveryEventHasAUniqueId(t *testing.T) {
	events := []Event{
		{Creator: 1},
		{Creator: 2},
		{Creator: 1, Parents: []*Event{{Creator: 1}}},
		{Creator: 1, Parents: []*Event{{Creator: 2}}},
		{Creator: 1, Parents: []*Event{{Creator: 1}, {Creator: 2}}},
	}

	for i, event := range events {
		id := event.EventId()
		for j, other := range events {
			if i == j {
				continue
			}
			require.NotEqual(t, id, other.EventId(), "Event %d should not have the same ID as Event %d", i, j)
		}
	}
}

func TestEvent_GenesisEvent_HasNoSelfParent(t *testing.T) {
	event := Event{Creator: 1}
	require.Nil(t, event.SelfParent(), "Genesis event should not have a self parent")
}

func TestEvent_FirstParentIsSelfParent(t *testing.T) {
	event := Event{
		Creator: 1,
		Parents: []*Event{{Creator: 2}, {Creator: 3}},
	}
	selfParent := event.SelfParent()
	require.NotNil(t, selfParent, "Event should have a self parent")
	require.Equal(t, Event{Creator: 2}.EventId(), selfParent.EventId(), "First parent should be the self parent")
}
