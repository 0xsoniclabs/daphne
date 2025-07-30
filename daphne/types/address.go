package types

import (
	"encoding/binary"
	"fmt"
)

// Address is used to identify accounts in the chain emulated by Daphne.
type Address uint64

func (a Address) String() string {
	return fmt.Sprintf("#%d", a)
}

// Serialize encodes the Address as a big-endian 8-byte value.
func (a *Address) Serialize() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(*a))
}

// Deserialize decodes a big-endian 8-byte value into an Address.
// It returns an error if the input data is not exactly 8 bytes long.
func (a *Address) Deserialize(data []byte) error {
	if len(data) != 8 {
		return fmt.Errorf("data must be exactly 8 bytes to deserialize Address")
	}
	*a = Address(binary.BigEndian.Uint64(data[:8]))
	return nil
}
