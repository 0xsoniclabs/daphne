// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

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
