package node

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestNode_NewRpcNode_CorrectlyInitializedRpcNode(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	node, err := NewRpcNode(p2p.PeerId("peer"), network)
	require.NoError(err)
	require.NotNil(node.GetRpcService())
	require.NotNil(node)
	require.Equal(p2p.PeerId("peer"), node.id)
}

func TestNode_NewRpcNode_ErrorOnNilNetwork(t *testing.T) {
	require := require.New(t)

	_, err := NewRpcNode(p2p.PeerId("peer"), nil)
	require.Error(err)
	require.ErrorContains(err, "network is nil")
}

func TestNode_NewRpcNode_ErrorOnDoubleStartedNode(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()
	peer := p2p.PeerId("peer")

	_, err := NewRpcNode(peer, network)
	require.NoError(err)

	_, err = NewRpcNode(peer, network)
	require.Error(err)
}

func TestNode_MultiNode_SyncsTransactionPools(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	node1, err := newNode(p2p.PeerId("node1"), network)
	require.NoError(err)

	node2, err := newNode(p2p.PeerId("node2"), network)
	require.NoError(err)

	tx := types.Transaction{From: 1}

	rpc1 := node1.GetRpcService()
	require.NoError(rpc1.Send(tx))
	require.True(rpc1.IsPending(tx.Hash()))

	rpc2 := node2.GetRpcService()
	require.True(rpc2.IsPending(tx.Hash()))
}
