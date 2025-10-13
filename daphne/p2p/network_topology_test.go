package p2p

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFullyConnectedTopology_ShouldConnect_ReturnsTrueForAllPeersOtherThanItself(t *testing.T) {
	topology := NewFullyConnectedTopology()

	peerA := PeerId("peer-A")
	peerB := PeerId("peer-B")
	peerC := PeerId("peer-C")

	require.True(t, topology.ShouldConnect(peerA, peerB), "should connect peer A to peer B")
	require.True(t, topology.ShouldConnect(peerB, peerA), "should connect peer B to peer A")
	require.True(t, topology.ShouldConnect(peerA, peerC), "should connect peer A to peer C")
	require.False(t, topology.ShouldConnect(peerA, peerA), "should allow a peer to connect to itself")
}
