package model

import (
	"github.com/0xsoniclabs/daphne/daphne/types"
)

type EventId types.Hash

func (c EventId) Serialize() []byte {
	return c[:]
}

type Event struct {
	Creator CreatorId
	Parents []EventId
	Payload []types.Transaction
}

func (e Event) EventId() EventId {
	data := []byte{}
	data = append(data, e.Creator.Serialize()...)
	for _, parent := range e.Parents {
		data = append(data, parent.Serialize()...)
	}
	return EventId(types.Sha256(data))
}

func (e Event) SelfParent() *EventId {
	if len(e.Parents) > 0 {
		return &e.Parents[0]
	}
	return nil
}

func (e Event) IsGenesis() bool {
	return len(e.Parents) == 0
}
