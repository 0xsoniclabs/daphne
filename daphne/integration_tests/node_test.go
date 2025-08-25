package integrationtests

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/node"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestNode_MultiNode_SyncsTransactionPools(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	node1, err := node.New(p2p.PeerId("node1"), network)
	require.NoError(err)

	node2, err := node.New(p2p.PeerId("node2"), network)
	require.NoError(err)

	tx := types.Transaction{From: 1}

	rpc1 := node1.GetRpcService()
	require.NoError(rpc1.Send(tx))
	require.True(rpc1.IsPending(tx.Hash()))

	rpc2 := node2.GetRpcService()

	time.Sleep(10 * time.Millisecond)

	require.True(rpc2.IsPending(tx.Hash()))
}
