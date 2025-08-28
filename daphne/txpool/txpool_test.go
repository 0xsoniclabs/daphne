package txpool

import (
	"errors"
	"sync"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTxPool_NewTxPool_CreatesEmptyPool(t *testing.T) {
	pool := NewTxPool()
	require.NotNil(t, pool)
	require.NotNil(t, pool.transactions)
	require.NotNil(t, pool.listeners)
	require.Empty(t, pool.transactions)
	require.Empty(t, pool.listeners)
}

func TestTxPool_Add_AddsTransactionToPool(t *testing.T) {
	pool := NewTxPool()
	tx := types.Transaction{
		From:  1,
		To:    2,
		Nonce: 0,
		Value: 100,
	}

	err := pool.Add(tx)
	require.NoError(t, err)

	transactions := pool.transactions[tx.From]
	require.Len(t, transactions, 1)
	require.Equal(t, tx, transactions[0])
}

func TestTxPool_Add_SortsTransactionsByNonce(t *testing.T) {
	pool := NewTxPool()
	from := types.Address(1)

	// Add transactions with nonces in non-sequential order
	tx1 := types.Transaction{From: from, Nonce: 2}
	tx2 := types.Transaction{From: from, Nonce: 0}
	tx3 := types.Transaction{From: from, Nonce: 1}

	require.NoError(t, pool.Add(tx1))
	require.NoError(t, pool.Add(tx2))
	require.NoError(t, pool.Add(tx3))

	transactions := pool.transactions[from]
	require.Len(t, transactions, 3)
	require.Equal(t, types.Nonce(0), transactions[0].Nonce)
	require.Equal(t, types.Nonce(1), transactions[1].Nonce)
	require.Equal(t, types.Nonce(2), transactions[2].Nonce)
}

func TestTxPool_Add_RejectsDuplicateNonceFromSameAddress(t *testing.T) {
	pool := NewTxPool()
	from := types.Address(1)

	tx1 := types.Transaction{From: from, To: 2, Nonce: 0}
	tx2 := types.Transaction{From: from, To: 3, Nonce: 0}

	err1 := pool.Add(tx1)
	require.NoError(t, err1)

	err2 := pool.Add(tx2)
	require.Error(t, err2)
	require.ErrorContains(t, err2, "transaction with nonce")
	require.ErrorContains(t, err2, "already exists")

	// Verify only the first transaction was added
	transactions := pool.transactions[from]
	require.Len(t, transactions, 1)
	require.Equal(t, tx1, transactions[0])
}

func TestTxPool_Add_AllowsSameNonceFromDifferentAddresses(t *testing.T) {
	pool := NewTxPool()

	tx1 := types.Transaction{From: 1, Nonce: 0}
	tx2 := types.Transaction{From: 2, Nonce: 0}

	err1 := pool.Add(tx1)
	require.NoError(t, err1)

	err2 := pool.Add(tx2)
	require.NoError(t, err2)

	require.Len(t, pool.transactions[1], 1)
	require.Len(t, pool.transactions[2], 1)
}

func TestTxPool_Add_NotifiesListeners(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pool := NewTxPool()
	listener1 := NewMockTxPoolListener(ctrl)
	listener2 := NewMockTxPoolListener(ctrl)

	pool.RegisterListener(listener1)
	pool.RegisterListener(listener2)

	tx := types.Transaction{
		From:  1,
		To:    2,
		Nonce: 0,
		Value: 100,
	}

	listener1.EXPECT().OnNewTransaction(tx).Times(1)
	listener2.EXPECT().OnNewTransaction(tx).Times(1)

	err := pool.Add(tx)
	require.NoError(t, err)
}

func TestTxPool_GetExecutableTransactions_ReturnsConsecutiveTransactions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pool := NewTxPool()
	source := NewMockNonceSource(ctrl)
	from := types.Address(1)

	// Set current nonce to 2
	source.EXPECT().GetNonce(from).Return(types.Nonce(2)).Times(1)

	// Add transactions with nonces 2, 3, 5
	tx1 := types.Transaction{From: from, Nonce: 2}
	tx2 := types.Transaction{From: from, Nonce: 3}
	tx3 := types.Transaction{From: from, Nonce: 5}

	require.NoError(t, pool.Add(tx1))
	require.NoError(t, pool.Add(tx2))
	require.NoError(t, pool.Add(tx3))

	executable := pool.GetExecutableTransactions(source)

	// Should return transactions with nonces 2 and 3, but not 5 (gap)
	require.Len(t, executable, 2)
	require.Equal(t, types.Nonce(2), executable[0].Nonce)
	require.Equal(t, types.Nonce(3), executable[1].Nonce)
}

func TestTxPool_GetExecutableTransactions_PrunesOutdatedTransactions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pool := NewTxPool()
	source := NewMockNonceSource(ctrl)
	from := types.Address(1)

	// Add transactions with nonces 0, 1, 2
	tx1 := types.Transaction{From: from, Nonce: 0}
	tx2 := types.Transaction{From: from, Nonce: 1}
	tx3 := types.Transaction{From: from, Nonce: 2}

	require.NoError(t, pool.Add(tx1))
	require.NoError(t, pool.Add(tx2))
	require.NoError(t, pool.Add(tx3))

	// Set current nonce to 2 (meaning 0 and 1 are outdated)
	source.EXPECT().GetNonce(from).Return(types.Nonce(2)).Times(1)

	executable := pool.GetExecutableTransactions(source)

	// Should return only transaction with nonce 2
	require.Len(t, executable, 1)
	require.Equal(t, types.Nonce(2), executable[0].Nonce)

	// Executable transactions should still be in the pool until processed
	// The pool only prunes outdated transactions (nonce < current), not executable ones
	remaining := pool.transactions[from]
	require.Len(t, remaining, 1)
	require.Equal(t, types.Nonce(2), remaining[0].Nonce)
}

func TestTxPool_GetExecutableTransactions_HandlesMultipleSenders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pool := NewTxPool()
	source := NewMockNonceSource(ctrl)

	from1 := types.Address(1)
	from2 := types.Address(2)

	source.EXPECT().GetNonce(from1).Return(types.Nonce(0)).Times(1)
	source.EXPECT().GetNonce(from2).Return(types.Nonce(1)).Times(1)

	// Add transactions from different senders
	tx1 := types.Transaction{From: from1, To: 3, Nonce: 0}
	tx2 := types.Transaction{From: from1, To: 4, Nonce: 1}
	tx3 := types.Transaction{From: from2, To: 5, Nonce: 1}
	tx4 := types.Transaction{From: from2, To: 6, Nonce: 2}

	require.NoError(t, pool.Add(tx1))
	require.NoError(t, pool.Add(tx2))
	require.NoError(t, pool.Add(tx3))
	require.NoError(t, pool.Add(tx4))

	executable := pool.GetExecutableTransactions(source)

	// Should return tx1, tx2 from sender1 and tx3, tx4 from sender2
	require.Len(t, executable, 4)

	// Verify all transactions are included
	foundNonces := make(map[types.Address][]types.Nonce)
	for _, tx := range executable {
		foundNonces[tx.From] = append(foundNonces[tx.From], tx.Nonce)
	}

	require.Contains(t, foundNonces, from1)
	require.Contains(t, foundNonces, from2)
	require.ElementsMatch(t, []types.Nonce{0, 1}, foundNonces[from1])
	require.ElementsMatch(t, []types.Nonce{1, 2}, foundNonces[from2])
}

func TestTxPool_GetExecutableTransactions_ReturnsEmptyWhenNoExecutableTransactions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pool := NewTxPool()
	source := NewMockNonceSource(ctrl)
	from := types.Address(1)

	// Set current nonce to 0, but add transaction with nonce 2 (gap)
	source.EXPECT().GetNonce(from).Return(types.Nonce(0)).Times(1)
	tx := types.Transaction{From: from, To: 2, Nonce: 2, Value: 100}

	require.NoError(t, pool.Add(tx))

	executable := pool.GetExecutableTransactions(source)
	require.Empty(t, executable)
}

func TestTxPool_RegisterListener_IgnoresNilListener(t *testing.T) {
	pool := NewTxPool()
	initialLen := len(pool.listeners)

	pool.RegisterListener(nil)

	require.Equal(t, initialLen, len(pool.listeners))
}

func TestTxPool_Contains_EmptyPool_ContainsNothing(t *testing.T) {
	pool := NewTxPool()
	require.Zero(t, len(pool.transactions))
	require.Zero(t, len(pool.listeners))
}

func TestTxPool_Contains_WithTransactions_ContainsReportsPresentTransactions(t *testing.T) {
	pool := NewTxPool()
	tx1 := types.Transaction{From: 1}
	tx2 := types.Transaction{From: 2}
	tx3 := types.Transaction{From: 3}

	require.False(t, pool.Contains(tx1.Hash()))
	require.False(t, pool.Contains(tx2.Hash()))
	require.False(t, pool.Contains(tx3.Hash()))

	require.NoError(t, pool.Add(tx1))

	require.True(t, pool.Contains(tx1.Hash()))
	require.False(t, pool.Contains(tx2.Hash()))
	require.False(t, pool.Contains(tx3.Hash()))

	require.NoError(t, pool.Add(tx2))

	require.True(t, pool.Contains(tx1.Hash()))
	require.True(t, pool.Contains(tx2.Hash()))
	require.False(t, pool.Contains(tx3.Hash()))
}

func TestTxPool_AddAndContains_AreThreadSafe(t *testing.T) {
	pool := NewTxPool()
	tx := types.Transaction{From: 1}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		require.NoError(t, pool.Add(tx))
	}()
	go func() {
		defer wg.Done()
		pool.Contains(tx.Hash())
	}()
	wg.Wait()
}

func TestTxGossip_AddingToOnePoolWithoutErrorCausesOtherPoolToReceiveTransaction(t *testing.T) {
	network := p2p.NewNetwork()

	server1, err := network.NewServer(p2p.PeerId("peer1"))
	require.NoError(t, err)

	server2, err := network.NewServer(p2p.PeerId("peer2"))
	require.NoError(t, err)

	pool1 := NewTxPool()
	pool2 := NewTxPool()
	InstallTxGossip(pool1, server1)
	InstallTxGossip(pool2, server2)

	tx := types.Transaction{From: 1}
	err = pool1.Add(tx)
	require.NoError(t, err)

	network.WaitForDeliveryOfSentMessages()

	require.True(t, pool2.Contains(tx.Hash()))
}

func TestTxGossip_AddingToOnePoolWithErrorDoesNotBroadcastTransaction(t *testing.T) {
	network := p2p.NewNetwork()

	server1, err := network.NewServer(p2p.PeerId("peer1"))
	require.NoError(t, err)

	server2, err := network.NewServer(p2p.PeerId("peer2"))
	require.NoError(t, err)

	pool1 := NewTxPool()
	// A transaction with nonce 0 already exists, so adding another will fail.
	err = pool1.Add(types.Transaction{Nonce: 0})
	require.NoError(t, err)
	pool2 := NewTxPool()
	InstallTxGossip(pool1, server1)
	InstallTxGossip(pool2, server2)

	tx := types.Transaction{Nonce: 0}
	err = pool1.Add(tx)
	require.Error(t, err)

	network.WaitForDeliveryOfSentMessages()

	require.False(t, pool2.Contains(tx.Hash()))
}

func TestInstallTxGossip_RegistersHandlers(t *testing.T) {
	ctrl := gomock.NewController(t)

	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())

	pool := NewMockTxPool(ctrl)
	pool.EXPECT().RegisterListener(gomock.Any())

	InstallTxGossip(pool, server)
}

func TestTxGossip_MessageReceiverAdapter_AddsTransactionToPoolSuccessfully(t *testing.T) {
	ctrl := gomock.NewController(t)
	pool := NewMockTxPool(ctrl)
	pool.EXPECT().Add(types.Transaction{}).Return(nil)

	adapter := messageReceiverAdapter{pool}
	adapter.OnMessage(types.Transaction{})
}

func TestTxGossip_HandleMessage_AddsTransactionToPoolWithError(t *testing.T) {
	ctrl := gomock.NewController(t)
	pool := NewMockTxPool(ctrl)
	pool.EXPECT().Add(types.Transaction{}).Return(errors.New("test error"))

	adapter := messageReceiverAdapter{pool}
	// Expect that the error is logged, but nothing else happens.
	adapter.OnMessage(types.Transaction{})
}
