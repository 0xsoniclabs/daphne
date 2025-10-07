package integrationtests

import (
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGossip_BroadcastWorksWithP2pServer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
			}, p2p.MessageCode_TxGossip_NewTransaction, nil)

			receiver := generic.NewMockBroadcastReceiver[p2p.PeerId](ctrl)
			for j := range servers {
				// a node may hear its own messages back from the network, but it
				// is not mandatory
				min := 1
				if i == j {
					min = 0
				}
				receiver.EXPECT().OnMessage(toPeerId(j + 1)).MinTimes(min)
			}

			gossip.RegisterReceiver(receiver)
			gossips[i] = gossip
		}

		for i := range 5 {
			gossips[i].Broadcast(toPeerId(i + 1))
		}

		synctest.Wait()
	})
}

func toPeerId(i int) p2p.PeerId {
	return p2p.PeerId(fmt.Sprintf("server%d", i))
}
