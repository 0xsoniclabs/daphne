package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoin_String_PrintsWithPrefix(t *testing.T) {
	tests := map[Coin]string{
		0:   "$0",
		1:   "$1",
		256: "$256",
	}

	for coin, expected := range tests {
		t.Run(fmt.Sprintf("%d", coin), func(t *testing.T) {
			require.Equal(t, expected, coin.String())
		})
	}
}

func TestCoin_Serialize_DeserializesAsBigEndian8byteValue(t *testing.T) {
	tests := map[Coin][]byte{
		0:   {0, 0, 0, 0, 0, 0, 0, 0},
		1:   {0, 0, 0, 0, 0, 0, 0, 1},
		256: {0, 0, 0, 0, 0, 0, 1, 0},
	}

	for coin, expected := range tests {
		t.Run(fmt.Sprintf("%d", coin), func(t *testing.T) {
			data := coin.Serialize()
			require.Equal(t, expected, data)

			var deserialized Coin
			err := deserialized.Deserialize(data)
			require.NoError(t, err)
			require.Equal(t, coin, deserialized)
		})
	}
}

func TestCoin_Deserialize_FailsOnWrongInputLength(t *testing.T) {
	tests := [][]byte{
		{0, 0, 0, 0, 0, 0, 0},       // too short
		{0, 0, 0, 0, 0, 0, 0, 1, 2}, // too long
	}

	for _, data := range tests {
		t.Run(fmt.Sprintf("%v", data), func(t *testing.T) {
			var coin Coin
			err := coin.Deserialize(data)
			require.Error(t, err)
		})
	}
}
