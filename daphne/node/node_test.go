package node

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNode_newBaseNode_InitializesCommonInfrastructure(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	server, rpc, provider, err := newBaseNode(p2p.PeerId("peer"), network)
	require.NoError(err)
	require.NotNil(server)
	require.NotNil(rpc)
	require.NotNil(provider)
}

func TestNode_newBaseNode_PropagatesNetworkError(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	_, _, _, err := newBaseNode(p2p.PeerId("peer"), network)
	require.NoError(err)

	_, _, _, err = newBaseNode(p2p.PeerId("peer"), network)
	require.Contains(err.Error(), "server with ID peer already exists")
}

func TestNode_NewActiveNode_InstantiatesActiveNode(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	ctrl := gomock.NewController(t)
	factory := consensus.NewMockFactory(ctrl)
	factory.EXPECT().NewActive(gomock.Any(), gomock.Any()).Return(consensus.NewMockConsensus(ctrl))
	node, err := NewActiveNode(p2p.PeerId("peer"), network, factory, nil)
	require.NoError(err)
	require.NotNil(node)
	require.NotNil(node.GetRpcService())
}

func TestNode_NewActiveNode_ErrorOnNilNetwork(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	factory := consensus.NewMockFactory(ctrl)

	node, err := NewActiveNode(p2p.PeerId("peer"), nil, factory, nil)
	require.Error(err)
	require.Nil(node)
}

func TestNode_NewPassiveNode_InstantiatesPassiveNode(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	ctrl := gomock.NewController(t)
	factory := consensus.NewMockFactory(ctrl)
	factory.EXPECT().NewPassive(gomock.Any()).Return(consensus.NewMockConsensus(ctrl))
	node, err := NewPassiveNode(p2p.PeerId("peer"), network, factory)
	require.NoError(err)
	require.NotNil(node)
	require.NotNil(node.GetRpcService())
}

func TestNode_NewPassiveNode_ErrorOnNilNetwork(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	factory := consensus.NewMockFactory(ctrl)

	node, err := NewPassiveNode(p2p.PeerId("peer"), nil, factory)
	require.Error(err)
	require.Nil(node)
}

func TestNode_GetCandidateTransactions_ReturnsExpectedTransactions(t *testing.T) {
	require := require.New(t)

	pool := txpool.NewTxPool()

	txs := []types.Transaction{
		{From: 1, Nonce: 0, Value: 100, To: 2},
		{From: 1, Nonce: 1, Value: 123, To: 3},
	}

	err := pool.Add(txs[0])
	require.NoError(err)
	err = pool.Add(txs[1])
	require.NoError(err)

	genesis := state.Genesis{
		1: {Nonce: 0, Balance: 100},
		2: {Nonce: 0, Balance: 50},
	}

	nodeState := state.New(genesis)
	provider := newTransactionProvider(nodeState, pool)
	candidates := provider.GetCandidateTransactions()
	require.Equal(txs, candidates)
}

func TestNode_Stop_ShutsDownNodeGracefully(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	ctrl := gomock.NewController(t)
	mockConsensus := consensus.NewMockConsensus(ctrl)
	mockConsensus.EXPECT().Stop()

	factory := consensus.NewMockFactory(ctrl)
	factory.EXPECT().NewActive(gomock.Any(), gomock.Any()).Return(mockConsensus)

	node, err := NewActiveNode(p2p.PeerId("peer"), network, factory, nil)
	require.NoError(err)
	require.NotNil(node)

	node.Stop()
}
