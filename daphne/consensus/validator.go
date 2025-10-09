package consensus

import "encoding/binary"

// ValidatorId represents the ID of a validator node in the network.
type ValidatorId uint32

func (c ValidatorId) Serialize() []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, uint32(c))
	return data
}
