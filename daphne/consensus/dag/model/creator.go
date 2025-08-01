package model

import "encoding/binary"

type CreatorId uint32

func (c CreatorId) Serialize() []byte {
	data := make([]byte, 4)
	return binary.BigEndian.AppendUint32(data, uint32(c))
}
