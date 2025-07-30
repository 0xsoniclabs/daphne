package types

import (
	"encoding/binary"
	"fmt"
)

// Coin is used to represent a monetary value in the chain emulated by Daphne.
type Coin uint64

func (c Coin) String() string {
	return fmt.Sprintf("$%d", c)
}

// Serialize encodes the Coin as a big-endian 8-byte value.
func (c *Coin) Serialize() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(*c))
}

// Deserialize decodes a big-endian 8-byte value into a Coin.
// It returns an error if the input data is not exactly 8 bytes long.
func (c *Coin) Deserialize(data []byte) error {
	if len(data) != 8 {
		return fmt.Errorf("data must be exactly 8 bytes to deserialize Coin")
	}
	*c = Coin(binary.BigEndian.Uint64(data[:8]))
	return nil
}
