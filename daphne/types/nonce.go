package types

import (
	"encoding/binary"
	"fmt"
)

// Nonce is used to track the number of transactions sent from an account in
// the chain emulated by Daphne.
type Nonce uint64

func (n Nonce) String() string {
	return fmt.Sprintf("nonce%d", n)
}

// Serialize encodes the Nonce as a big-endian 8-byte value.
func (n *Nonce) Serialize() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(*n))
}

// Deserialize decodes a big-endian 8-byte value into a Nonce.
// It returns an error if the input data is not exactly 8 bytes long.
func (n *Nonce) Deserialize(data []byte) error {
	if len(data) != 8 {
		return fmt.Errorf("data must be exactly 8 bytes to deserialize Nonce")
	}
	*n = Nonce(binary.BigEndian.Uint64(data[:8]))
	return nil
}
