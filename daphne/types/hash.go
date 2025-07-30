package types

import (
	"crypto/sha256"
	"fmt"
)

// Hash is a cryptographic digest of a value type.
type Hash [32]byte

// String returns the hexadecimal representation of the Hash.
func (h Hash) String() string {
	return fmt.Sprintf("0x%x", h[:])
}

// Sha256 computes the SHA-256 hash of the input data and returns it as a Hash.
func Sha256(data []byte) Hash {
	return Hash(sha256.Sum256(data))
}
