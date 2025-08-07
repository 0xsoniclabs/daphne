package p2p

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageMatcher_WithPayload_MatchesWithAnotherMessagePayload(t *testing.T) {
	payload := "test payload"
	matcher := WithPayload(payload)

	require.True(t, matcher.Matches(Message{Payload: payload}))
}

func TestMessageMatcher_WithPayload_DoesntMatchWithAnotherMessagePayload(t *testing.T) {
	matcher := WithPayload("test payload 1")

	require.False(t, matcher.Matches(Message{Payload: "test payload 2"}))
}

func TestMessageMatcher_WithPayload_DoesntMatchWithAnotherNonMessage(t *testing.T) {
	matcher := WithPayload("test payload 1")

	require.False(t, matcher.Matches(struct{}{}))
}

func TestMessageMatcher_String_PrintsMessageStringPayload(t *testing.T) {
	matcher := WithPayload("test payload")

	require.Equal(t, "Message with payload: is equal to test payload (string)", matcher.String())
}
