package node

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
)

func TestNode_NewNode_CorrectlyInitializesFreshNode(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	node, err := New(p2p.PeerId("peer"), network)
	require.NoError(err)
	require.NotNil(node)
	require.NotNil(node.GetRpcService())
	require.Equal(p2p.PeerId("peer"), node.id)
}

func TestNode_NewNode_ErrorOnNilNetwork(t *testing.T) {
	require := require.New(t)

	_, err := New(p2p.PeerId("peer"), nil)
	require.ErrorContains(err, "network is nil")
}

func TestNode_NewNode_ErrorOnDoubleStartedNode(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()
	peer := p2p.PeerId("peer")

	_, err := New(peer, network)
	require.NoError(err)

	_, err = New(peer, network)
	require.Error(err)
}
