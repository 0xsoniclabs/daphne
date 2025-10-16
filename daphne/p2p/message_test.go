package p2p

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
	"github.com/stretchr/testify/require"
)

func TestGetMessageType_VariousTypes_UsesTypeName(t *testing.T) {
	require := require.New(t)
	require.EqualValues("int", GetMessageType(42))
	require.EqualValues("string", GetMessageType("hello"))
	require.EqualValues("Set[int]", GetMessageType(sets.Set[int]{}))

	type myStruct struct{}
	require.EqualValues("myStruct", GetMessageType(myStruct{}))
}

func TestGetMessageType_NamedType_UsesTypeMethod(t *testing.T) {
	require := require.New(t)

	msg := myMessage{}
	require.EqualValues(msg.MessageType(), GetMessageType(msg))
}

type myMessage struct{}

func (m myMessage) MessageType() MessageType {
	return "Some type name determined by the type"
}
