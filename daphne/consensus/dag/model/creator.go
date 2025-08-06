package model

import "encoding/binary"

// CreatorId represents the ID of the creator of an event,
// which is a validator node on the network.
type CreatorId uint32

func (c CreatorId) Serialize() []byte {
	data := make([]byte, 4)
	return binary.BigEndian.AppendUint32(data, uint32(c))
}
