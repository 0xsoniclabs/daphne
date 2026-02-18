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

package broadcast

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestNewDefault_UsesDefaultFactoryForAllKeyMessageCombinations(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	intToInt := func(int) int { return 0 }
	require.IsType(
		DefaultFactory[int, int]()(p2pServer, intToInt),
		NewDefault(p2pServer, intToInt),
	)

	intToString := func(int) string { return "" }
	require.IsType(
		DefaultFactory[string, int]()(p2pServer, intToString),
		NewDefault(p2pServer, intToString),
	)

	boolToString := func(bool) string { return "" }
	require.IsType(
		DefaultFactory[string, bool]()(p2pServer, boolToString),
		NewDefault(p2pServer, boolToString),
	)
}
