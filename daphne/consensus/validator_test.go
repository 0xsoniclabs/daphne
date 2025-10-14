package consensus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidator_Serialize(t *testing.T) {
	validatorId := ValidatorId(10000)
	serialized := validatorId.Serialize()
	expected := []byte{0x00, 0x00, 0x27, 0x10} // 10000 in big-endian format

	require.Equal(t, expected, serialized, "Serialized data should match expected byte slice")
}
