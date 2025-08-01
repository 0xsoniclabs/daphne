package dag

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/node"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestDag_NetworkWithThreeNodes_CanProcessTransactions(t *testing.T) {
	require := require.New(t)

	id1 := p2p.PeerId("node1")
	id2 := p2p.PeerId("node2")
	id3 := p2p.PeerId("node3")

	network := p2p.NewNetwork()

	genesis := map[types.Address]state.Account{
		1: {Balance: 100},
	}

	_, err := node.NewValidator(id1, genesis, network, Algorithm{Creator: 1})
	require.NoError(err)
	_, err = node.NewValidator(id2, genesis, network, Algorithm{Creator: 2})
	require.NoError(err)
	_, err = node.NewValidator(id3, genesis, network, Algorithm{Creator: 3})
	require.NoError(err)

	node1, err := node.NewRpc(p2p.PeerId("rpc1"), genesis, network, Algorithm{})
	require.NoError(err)

	node2, err := node.NewRpc(p2p.PeerId("rpc2"), genesis, network, Algorithm{})
	require.NoError(err)
	tx := types.Transaction{From: 1, To: 2, Value: 10}

	rpc1 := node1.GetRpcService()
	require.NoError(rpc1.Send(tx))

	time.Sleep(2 * time.Second) // Allow time for gossip propagation

	rpc2 := node2.GetRpcService()
	receipt, err := rpc2.GetReceipt(tx.Hash())
	require.NoError(err)
	require.NotNil(receipt)
	require.True(receipt.Success)
}
