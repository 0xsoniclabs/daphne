package mark

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMark_String_ProducesReadableText(t *testing.T) {
	require := require.New(t)

	require.Equal("MsgSent", MsgSent.String())
	require.Equal("MsgReceived", MsgReceived.String())
	require.Equal("Mark(12345)", Mark(12345).String())
}
