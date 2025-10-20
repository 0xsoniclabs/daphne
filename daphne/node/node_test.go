package node

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNode_newBaseNode_InitializesCommonInfrastructure(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	tracker := tracker.NewMockTracker(ctrl)

	tracker.EXPECT().With(gomock.Any(), gomock.Any()).Return(tracker)

	config := NodeConfig{
		Network: p2p.NewNetwork(),
		Tracker: tracker,
	}

	server, rpc, provider, state, err := newBaseNode(
		p2p.PeerId("peer"),
		config,
	)
	require.NoError(err)
	require.NotNil(server)
	require.NotNil(rpc)
	require.NotNil(provider)
	require.NotNil(state)
}

func TestNode_newBaseNode_PropagatesNetworkError(t *testing.T) {
	require := require.New(t)

	config := NodeConfig{
		Network: p2p.NewNetwork(),
	}

	_, _, _, _, err := newBaseNode(p2p.PeerId("peer"), config)
	require.NoError(err)

	_, _, _, _, err = newBaseNode(p2p.PeerId("peer"), config)
	require.Contains(err.Error(), "server with ID peer already exists")
}

func TestNode_NewActiveNode_InstantiatesActiveNode(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)

	active := consensus.NewMockConsensus(ctrl)
	active.EXPECT().RegisterListener(gomock.Any())

	factory := consensus.NewMockFactory(ctrl)
	factory.EXPECT().NewActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(active)

	config := NodeConfig{
		Network:   p2p.NewNetwork(),
		Consensus: factory,
	}

	node, err := NewActiveNode(p2p.PeerId("peer"), 1, config)
	require.NoError(err)
	require.NotNil(node)
	require.NotNil(node.GetRpcService())
}

func TestNode_NewActiveNode_ErrorOnNilNetwork(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	factory := consensus.NewMockFactory(ctrl)

	node, err := NewActiveNode(p2p.PeerId("peer"), 1, NodeConfig{
		Consensus: factory,
	})
	require.Error(err)
	require.Nil(node)
}

func TestNode_NewNode_AppliesStateOnBundle(t *testing.T) {
	require := require.New(t)

	tests := map[string]struct {
		setupFactory func(
			ctrl *gomock.Controller,
			capturedListener *consensus.BundleListener,
		) consensus.Factory
		newNode func(
			p2p.PeerId,
			NodeConfig,
		) (*Node, error)
	}{
		"ActiveNode applies state on bundle": {
			setupFactory: func(
				ctrl *gomock.Controller,
				capturedListener *consensus.BundleListener,
			) consensus.Factory {
				active := consensus.NewMockConsensus(ctrl)
				active.EXPECT().
					RegisterListener(gomock.Any()).
					Do(func(l consensus.BundleListener) {
						*capturedListener = l
					})

				factory := consensus.NewMockFactory(ctrl)
				factory.EXPECT().NewActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(active)
				return factory
			},
			newNode: func(pi p2p.PeerId, nc NodeConfig) (*Node, error) {
				return NewActiveNode(pi, 1, nc)
			},
		},
		"PassiveNode applies state on bundle": {
			setupFactory: func(
				ctrl *gomock.Controller,
				capturedListener *consensus.BundleListener,
			) consensus.Factory {
				passive := consensus.NewMockConsensus(ctrl)
				passive.EXPECT().
					RegisterListener(gomock.Any()).
					Do(func(l consensus.BundleListener) {
						*capturedListener = l
					})

				factory := consensus.NewMockFactory(ctrl)
				factory.EXPECT().NewPassive(gomock.Any()).Return(passive)
				return factory
			},
			newNode: NewPassiveNode,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var capturedListener consensus.BundleListener
			factory := testCase.setupFactory(ctrl, &capturedListener)

			network := p2p.NewNetwork()
			genesis := state.Genesis{
				1: {Nonce: 0, Balance: 100},
				2: {Nonce: 0, Balance: 50},
			}

			mockTracker := tracker.NewMockTracker(ctrl)
			mockTracker.EXPECT().With(gomock.Any(), gomock.Any()).Return(mockTracker)

			txs := []types.Transaction{
				{From: 1, Nonce: 0, Value: 100, To: 2},
				{From: 1, Nonce: 1, Value: 123, To: 3},
			}
			bundle := types.Bundle{Transactions: txs}

			gomock.InOrder(
				mockTracker.EXPECT().Track(mark.TxConfirmed, "hash", txs[0].Hash(), "block", uint32(0)),
				mockTracker.EXPECT().Track(mark.TxBeginProcessing, "hash", txs[0].Hash(), "block", uint32(0)),
				mockTracker.EXPECT().Track(mark.TxEndProcessing, "hash", txs[0].Hash(), "block", uint32(0)),
				mockTracker.EXPECT().Track(mark.TxFinalized, "hash", txs[0].Hash(), "block", uint32(0)),
			)
			gomock.InOrder(
				mockTracker.EXPECT().Track(mark.TxConfirmed, "hash", txs[1].Hash(), "block", uint32(0)),
				mockTracker.EXPECT().Track(mark.TxBeginProcessing, "hash", txs[1].Hash(), "block", uint32(0)),
				mockTracker.EXPECT().Track(mark.TxEndProcessing, "hash", txs[1].Hash(), "block", uint32(0)),
				mockTracker.EXPECT().Track(mark.TxFinalized, "hash", txs[1].Hash(), "block", uint32(0)),
			)

			node, err := testCase.newNode(
				p2p.PeerId("peer"),
				NodeConfig{
					Network:   network,
					Consensus: factory,
					Genesis:   genesis,
					Tracker:   mockTracker,
				},
			)
			require.NoError(err)
			require.NotNil(node)
			require.NotNil(node.GetRpcService())

			require.NotNil(capturedListener, "expected RegisterListener to capture a listener")

			capturedListener.OnNewBundle(bundle)
		})
	}
}

func TestNode_NewPassiveNode_InstantiatesPassiveNode(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	ctrl := gomock.NewController(t)

	passive := consensus.NewMockConsensus(ctrl)
	passive.EXPECT().RegisterListener(gomock.Any())

	factory := consensus.NewMockFactory(ctrl)
	factory.EXPECT().NewPassive(gomock.Any()).Return(passive)
	node, err := NewPassiveNode(p2p.PeerId("peer"), NodeConfig{
		Network:   network,
		Consensus: factory,
	})
	require.NoError(err)
	require.NotNil(node)
	require.NotNil(node.GetRpcService())
}

func TestNode_NewPassiveNode_ErrorOnNilNetwork(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	factory := consensus.NewMockFactory(ctrl)

	node, err := NewPassiveNode(p2p.PeerId("peer"), NodeConfig{
		Consensus: factory,
	})
	require.Error(err)
	require.Nil(node)
}

func TestNode_GetCandidateTransactions_ReturnsExpectedTransactions(t *testing.T) {
	require := require.New(t)

	pool := txpool.NewTxPool(nil)

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

	nodeState := state.NewState(genesis)
	provider := newTransactionProvider(nodeState, pool)
	candidates := provider.GetCandidateTransactions()
	require.Equal(txs, candidates)
}

func TestNode_Stop_ShutsDownNodeGracefully(t *testing.T) {
	require := require.New(t)

	network := p2p.NewNetwork()

	ctrl := gomock.NewController(t)
	mockConsensus := consensus.NewMockConsensus(ctrl)
	mockConsensus.EXPECT().RegisterListener(gomock.Any()).AnyTimes()
	mockConsensus.EXPECT().Stop()

	factory := consensus.NewMockFactory(ctrl)
	factory.EXPECT().NewActive(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockConsensus)

	node, err := NewActiveNode(p2p.PeerId("peer"), 1, NodeConfig{
		Network:   network,
		Consensus: factory,
	})
	require.NoError(err)
	require.NotNil(node)

	node.Stop()
}
