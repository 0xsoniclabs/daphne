package integrationtests

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
)

// testReceiver is a simple implementation of BroadcastReceiver for testing purposes.
type testReceiver struct {
	f func(message p2p.PeerId)
}

func (r *testReceiver) OnMessage(message p2p.PeerId) {
	if r.f != nil {
		r.f(message)
	}
}

func TestGossip_BroadcastWorksWithP2pServer(t *testing.T) {
	// This test checks if the gossip broadcast works correctly with the P2P server.

	network := p2p.NewNetwork()
	servers := make([]p2p.Server, 5)
	for i := range 5 {
		server, err := network.NewServer(p2p.PeerId(fmt.Sprintf("%d", i+1)))
		require.NoError(t, err, "Failed to create server %d", i+1)
		servers[i] = server
	}

	// Keeps track of which messages each peer has received.
	peerReceivedRecord := make(map[p2p.PeerId]map[p2p.PeerId]struct{})

	gossips := make([]generic.Broadcaster[p2p.PeerId], 5)
	for i, server := range servers {
		gossip := generic.NewGossip(server, func(msg p2p.PeerId) p2p.PeerId {
			return msg
		}, p2p.MessageCode_TxGossip_NewTransaction)
		gossip.RegisterReceiver(&testReceiver{
			f: func(message p2p.PeerId) {
				myId := p2p.PeerId(fmt.Sprintf("%d", i+1))
				if _, exists := peerReceivedRecord[myId]; !exists {
					peerReceivedRecord[myId] = make(map[p2p.PeerId]struct{})
				}
				peerReceivedRecord[myId][p2p.PeerId(message)] = struct{}{}
			},
		})
		gossips[i] = gossip
	}

	for i := range 5 {
		gossips[i].Broadcast(p2p.PeerId(fmt.Sprintf("%d", i+1)))
	}

	for i := range 5 {
		// Each peer should have received messages from all other peers.
		require.ElementsMatch(t, []p2p.PeerId{"1", "2", "3", "4", "5"},
			slices.Collect(maps.Keys(peerReceivedRecord[p2p.PeerId(fmt.Sprintf("%d", i+1))])))
	}
}
