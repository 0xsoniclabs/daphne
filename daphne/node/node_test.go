package node

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestNode_SyncTransactionPool(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	id1 := p2p.PeerId("node1")
	id2 := p2p.PeerId("node2")

	node1, err := NewNode(id1, network)
	require.NoError(err)

	node2, err := NewNode(id2, network)
	require.NoError(err)

	tx := types.Transaction{
		From:  1,
		To:    2,
		Value: 100,
	}

	rpc1 := node1.GetRpcService()
	require.NoError(rpc1.Send(tx))
	require.True(rpc1.IsPending(tx.Hash()))

	rpc2 := node2.GetRpcService()
	require.True(rpc2.IsPending(tx.Hash()))
}
