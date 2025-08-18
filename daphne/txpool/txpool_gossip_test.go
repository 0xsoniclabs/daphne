package txpool

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestTxGossip_InstallTxGossip_RegistersHandlers(t *testing.T) {
	ctrl := gomock.NewController(t)

	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())

	pool := NewMockTxPool(ctrl)
	pool.EXPECT().RegisterListener(gomock.Any())

	InstallTxGossip(pool, server)
}

func TestTxGossip_HandleMessage_RejectsInvalidMessageCode(t *testing.T) {
	txGossip, _, pool := newTestTxGossip(t)

	// Should not be called
	pool.EXPECT().Add(gomock.Any()).Times(0)
	txGossip.HandleMessage(p2p.PeerId("peer"), p2p.Message{
		Code: p2p.MessageCode_TxGossip_NewTransaction + 100, // Invalid code
	})
}

func TestTxGossip_HandleMessage_RejectsInvalidMessagePayload(t *testing.T) {
	txGossip, _, pool := newTestTxGossip(t)

	// Should not be called
	pool.EXPECT().Add(gomock.Any()).Times(0)
	txGossip.HandleMessage(p2p.PeerId("peer"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: "invalid payload", // Invalid payload type
	})
}

func TestTxGossip_HandleMessage_ForwardsValidTxToPool(t *testing.T) {
	txGossip, server, pool := newTestTxGossip(t)

	// No outbound connections as we are not interested in gossiping transactions
	server.EXPECT().GetPeers().Return([]p2p.PeerId{})
	tx := types.Transaction{From: 1}
	pool.EXPECT().Add(tx)

	txGossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: tx,
	})
}

func TestTxGossip_HandleMessage_BroadcastTxRejectedByPool(t *testing.T) {
	txGossip, server, pool := newTestTxGossip(t)

	pool.EXPECT().Add(gomock.Any()).Return(fmt.Errorf("rejected by pool"))
	// The transaction should be broadcasted further, even if it was rejected by the pool.
	server.EXPECT().GetPeers()

	txGossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: types.Transaction{},
	})
}

func TestTxGossip_OnNewTransaction_SendOverTheNetworkFails(t *testing.T) {
	txGossip, server, _ := newTestTxGossip(t)

	// Following scenario should only trigger a warning log
	server.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1"})
	server.EXPECT().SendMessage(p2p.PeerId("peer1"), gomock.Any()).Return(fmt.Errorf("network error"))
	txGossip.OnNewTransaction(types.Transaction{})
}

func TestTxGossip_OnNewTransaction_GossipsTxsToNewPeers(t *testing.T) {
	txGossip, server, _ := newTestTxGossip(t)

	tx1 := types.Transaction{From: 1}
	// Initially the only connected peer is "peer1"
	server.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1"})
	// The tx gets sent only to "peer1".
	server.EXPECT().SendMessage(p2p.PeerId("peer1"), p2p.WithPayload(tx1))
	txGossip.OnNewTransaction(tx1)

	// "peer2" joins the network
	server.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"})
	// The same transaction should only be sent to new peers.
	server.EXPECT().SendMessage(p2p.PeerId("peer2"), p2p.WithPayload(tx1))
	txGossip.OnNewTransaction(tx1)

	tx2 := types.Transaction{From: 2}
	server.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"})
	// A new transaction should be sent to both peers.
	server.EXPECT().SendMessage(p2p.PeerId("peer1"), p2p.WithPayload(tx2))
	server.EXPECT().SendMessage(p2p.PeerId("peer2"), p2p.WithPayload(tx2))
	txGossip.OnNewTransaction(tx2)
}

func TestTxGossip_OnNewTransaction_SkipGossipForPeersFromWhichTxWasReceived(t *testing.T) {
	txGossip, server, pool := newTestTxGossip(t)

	tx := types.Transaction{From: 1}

	pool.EXPECT().Add(gomock.Any()).Return(nil)
	server.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"})

	// The transaction should be sent to "peer2" only
	server.EXPECT().SendMessage(p2p.PeerId("peer2"), p2p.WithPayload(tx))
	txGossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: tx,
	})

	server.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"})
	// Subsequent notifications about the same transaction shoud not trigger any new gossips.
	txGossip.OnNewTransaction(tx)
}

func TestTxGossip_MultiNode_GossipsNewTransactionToPeers(t *testing.T) {
	require := require.New(t)
	network := p2p.NewNetwork(nil)

	server1, err := network.NewServer(p2p.PeerId("peer1"))
	require.NoError(err)

	server2, err := network.NewServer(p2p.PeerId("peer2"))
	require.NoError(err)

	pool1 := NewTxPool()
	pool2 := NewTxPool()
	InstallTxGossip(pool1, server1)
	InstallTxGossip(pool2, server2)

	tx := types.Transaction{From: 1}
	err = pool1.Add(tx)
	require.NoError(err)

	pool2.Contains(tx.Hash())
}

// newTestTxGossip manually creates a new txGossip instance with mocked dependencies.
// It is useful for testing as `InstallTxGossip` keeps no explicit reference to the txGossip instance.
func newTestTxGossip(t *testing.T) (*txGossip, *p2p.MockServer, *MockTxPool) {
	t.Helper()
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	pool := NewMockTxPool(ctrl)

	txGossip := &txGossip{
		p2p:                      server,
		pool:                     pool,
		transactionsKnownByPeers: make(map[p2p.PeerId]map[types.Hash]struct{}),
	}

	return txGossip, server, pool
}
