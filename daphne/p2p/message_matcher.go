package p2p

import (
	"fmt"

	gomock "go.uber.org/mock/gomock"
)

// WithPayload is a gomock matcher for p2p.Message instances that checks whether
// a given message has a specified payload. The payload can be any type that
// implements the gomock.Matcher interface, or a specific value that should be
// matched exactly.
func WithPayload(payload any) gomock.Matcher {
	matcher, ok := payload.(gomock.Matcher)
	if ok {
		return withPayload{payload: matcher}
	}
	return WithPayload(gomock.Eq(payload))
}

type withPayload struct {
	payload gomock.Matcher
}

func (m withPayload) Matches(arg any) bool {
	msg, ok := arg.(Message)
	if !ok {
		return false
	}
	return m.payload.Matches(msg.Payload)
}

func (m withPayload) String() string {
	return fmt.Sprintf("Message with payload: %s", m.payload.String())
}
