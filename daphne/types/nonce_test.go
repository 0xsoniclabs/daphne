package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNonce_String_PrintsAsString(t *testing.T) {
	tests := map[Nonce]string{
		0:   "0",
		1:   "1",
		256: "256",
	}

	for nonce, expected := range tests {
		t.Run(fmt.Sprintf("%d", nonce), func(t *testing.T) {
			require.Equal(t, expected, nonce.String())
		})
	}
}

func TestNonce_Serialize_DeserializesAsBigEndian8byteValue(t *testing.T) {
	tests := map[Nonce][]byte{
		1:   {0, 0, 0, 0, 0, 0, 0, 1},
		15:  {0, 0, 0, 0, 0, 0, 0, 15},
		512: {0, 0, 0, 0, 0, 0, 2, 0},
	}

	for nonce, expected := range tests {
		t.Run(fmt.Sprintf("%d", nonce), func(t *testing.T) {
			data := nonce.Serialize()
			require.Equal(t, expected, data)

			var deserialized Nonce
			err := deserialized.Deserialize(data)
			require.NoError(t, err)
			require.Equal(t, nonce, deserialized)
		})
	}
}

func TestNonce_Deserialize_FailsOnWrongInputLength(t *testing.T) {
	tests := [][]byte{
		{0, 0, 0, 0, 0, 0, 0},       // too short
		{0, 0, 0, 0, 0, 0, 1, 2, 3}, // too long
	}

	for _, data := range tests {
		t.Run(fmt.Sprintf("%v", data), func(t *testing.T) {
			var nonce Nonce
			err := nonce.Deserialize(data)
			require.Error(t, err)
		})
	}
}
