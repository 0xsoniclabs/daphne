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

func TestNewChannel_ProducesAnInstanceOfTheSpecifiedType(t *testing.T) {
	tests := map[Protocol]any{
		ProtocolGossip:     &gossip[int, int]{},
		ProtocolForwarding: &forwarding[int, int]{},
	}

	for protocol, expectedType := range tests {
		t.Run(protocol.String(), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			p2pServer := p2p.NewMockServer(ctrl)
			p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

			intToInt := func(int) int { return 0 }
			channel := NewChannel(protocol, p2pServer, intToInt)
			require.IsType(t, expectedType, channel)
		})
	}
}

func TestNewChannel_UnknownProtocol_Panics(t *testing.T) {
	intToInt := func(int) int { return 0 }
	require.Panics(t, func() {
		NewChannel(Protocol(999), nil, intToInt)
	})
}
