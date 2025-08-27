package integrationtests

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestTxPool_AddingToOnePoolCausesOtherPoolToReceiveTransaction(t *testing.T) {
	network := p2p.NewNetwork()

	server1, err := network.NewServer(p2p.PeerId("peer1"))
	require.NoError(t, err)

	server2, err := network.NewServer(p2p.PeerId("peer2"))
	require.NoError(t, err)

	pool1 := txpool.NewTxPool()
	pool2 := txpool.NewTxPool()
	txpool.InstallTxGossip(pool1, server1)
	txpool.InstallTxGossip(pool2, server2)

	tx := types.Transaction{From: 1}
	err = pool1.Add(tx)
	require.NoError(t, err)

	require.True(t, pool2.Contains(tx.Hash()))
}
