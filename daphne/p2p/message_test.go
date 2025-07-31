package p2p

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageCode_String_ProducesLabelWithoutPrefix(t *testing.T) {
	tests := map[MessageCode]string{
		MessageCode_UnitTestProtocol_Ping: "UnitTestProtocol_Ping",
	}

	for code, expected := range tests {
		t.Run(expected, func(t *testing.T) {
			require.Equal(t, expected, code.String())
		})
	}
}
