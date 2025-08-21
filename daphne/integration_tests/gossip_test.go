package integrationtests

import (
	"fmt"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

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

	// Keeps track of which peer has received messages from which peers.
	mu := sync.Mutex{}
	peerReceivedRecord := make(map[p2p.PeerId]map[p2p.PeerId]struct{})

	gossips := make([]generic.Broadcaster[p2p.PeerId], 5)
	for i, server := range servers {
		gossip := generic.NewGossip(server, func(msg p2p.PeerId) p2p.PeerId {
			return msg
		}, p2p.MessageCode_TxGossip_NewTransaction)
		gossip.RegisterReceiver(&testReceiver{
			f: func(message p2p.PeerId) {
				mu.Lock()
				defer mu.Unlock()

				myId := toServerId(i + 1)
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

	time.Sleep(10 * time.Millisecond)
	for _, s := range servers {
		s.Close()
	}

	for i := range 5 {
		// Each peer should have received messages from all other peers,
		// except for the original message sent from itself.
		expected := []p2p.PeerId{"1", "2", "3", "4", "5"}
		expected = slices.DeleteFunc(expected, func(id p2p.PeerId) bool {
			return id == toServerId(i+1)
		})

		mu.Lock()
		received := slices.Collect(maps.Keys(peerReceivedRecord[toServerId(i+1)]))
		mu.Unlock()

		for _, id := range expected {
			require.Contains(t, received, id)
		}
	}
}

func toServerId(i int) p2p.PeerId {
	return p2p.PeerId(fmt.Sprintf("%d", i))
}
