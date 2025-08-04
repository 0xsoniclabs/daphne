package model

import (
	"github.com/0xsoniclabs/daphne/daphne/types"
)

type EventId types.Hash

func (c EventId) Serialize() []byte {
	return c[:]
}

type EventMessage struct {
	Creator CreatorId
	Parents []EventId
	Payload []types.Transaction
}

type Event struct {
	Seq     CreatorId
	Creator CreatorId
	Parents []*Event
	Payload []types.Transaction
}

func (e Event) EventId() EventId {
	return e.ToEventMessage().EventId()
}

func (e EventMessage) EventId() EventId {
	data := []byte{}
	data = append(data, e.Creator.Serialize()...)
	for _, parent := range e.Parents {
		data = append(data, parent.Serialize()...)
	}
	return EventId(types.Sha256(data))
}

func (e *Event) ToEventMessage() EventMessage {
	parents := make([]EventId, len(e.Parents))
	for i, parent := range e.Parents {
		if parent == nil {
			continue
		}
		parents[i] = parent.EventId()
	}
	return EventMessage{
		Creator: e.Creator,
		Parents: parents,
		Payload: e.Payload,
	}
}

func (e Event) SelfParent() *Event {
	if len(e.Parents) > 0 {
		return e.Parents[0]
	}
	return nil
}

func (e Event) IsGenesis() bool {
	return len(e.Parents) == 0
}
