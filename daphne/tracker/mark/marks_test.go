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
