package integrationtests

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGossip_BroadcastWorksWithP2pServer(t *testing.T) {
	// This test checks if the gossip broadcast works correctly with the P2P server.

	network := p2p.NewNetwork()
	servers := make([]p2p.Server, 5)
	for i := range 5 {
		server, err := network.NewServer(toPeerId(i + 1))
		require.NoError(t, err, "Failed to create server %d", i+1)
		servers[i] = server
	}

	ctrl := gomock.NewController(t)

	gossips := make([]generic.Broadcaster[p2p.PeerId], 5)
	for i, server := range servers {
		gossip := generic.NewGossip(server, func(msg p2p.PeerId) p2p.PeerId {
			return msg
		}, p2p.MessageCode_TxGossip_NewTransaction)

		receiver := generic.NewMockBroadcastReceiver[p2p.PeerId](ctrl)

		minimum := func(j int) int {
			// a node may hear its own messages back from the network, but it
			// is not mandatory
			if i+1 == j {
				return 0
			}
			// messages originated from another id
			// must be received at least once
			return 1
		}

		receiver.EXPECT().OnMessage(toPeerId(1)).MinTimes(minimum(1))
		receiver.EXPECT().OnMessage(toPeerId(2)).MinTimes(minimum(2))
		receiver.EXPECT().OnMessage(toPeerId(3)).MinTimes(minimum(3))
		receiver.EXPECT().OnMessage(toPeerId(4)).MinTimes(minimum(4))
		receiver.EXPECT().OnMessage(toPeerId(5)).MinTimes(minimum(5))

		gossip.RegisterReceiver(receiver)
		gossips[i] = gossip
	}

	for i := range 5 {
		gossips[i].Broadcast(toPeerId(i + 1))
	}

	network.WaitForAllMessagesBeingDelivered()
}

func toPeerId(i int) p2p.PeerId {
	return p2p.PeerId(fmt.Sprintf("server%d", i))
}
