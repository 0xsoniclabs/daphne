package integrationtests

import (
	"testing"
	"testing/synctest"

	"github.com/0xsoniclabs/daphne/daphne/consensus/central"
	"github.com/0xsoniclabs/daphne/daphne/node"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestNode_MultiNode_SyncsTransactionPools(t *testing.T) {
	require := require.New(t)

	config := node.NodeConfig{
		Network:   p2p.NewNetwork(),
		Consensus: central.Factory{},
	}

	node1, err := node.NewActiveNode(p2p.PeerId("node1"), 1, config)
	require.NoError(err)

	node2, err := node.NewActiveNode(p2p.PeerId("node2"), 2, config)
	require.NoError(err)

	tx := types.Transaction{From: 1}

	synctest.Test(t, func(t *testing.T) {
		rpc1 := node1.GetRpcService()
		require.NoError(rpc1.Send(tx))
		require.True(rpc1.IsPending(tx.Hash()))

		synctest.Wait()

		rpc2 := node2.GetRpcService()
		require.True(rpc2.IsPending(tx.Hash()))
	})
}
