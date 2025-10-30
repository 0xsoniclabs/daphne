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
