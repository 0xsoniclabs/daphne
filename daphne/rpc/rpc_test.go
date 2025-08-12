package rpc

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/receipt_store"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestServer_Send_ForwardTransactionToPool(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	pool := txpool.NewMockTxPool(ctrl)

	tx := types.Transaction{From: 1}
	pool.EXPECT().Add(tx).Times(1)

	server := NewServer(pool, nil)
	require.NoError(server.Send(tx))
}

func TestServer_IsPending_RequestsPresenceOfTransactionInPool(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	pool := txpool.NewMockTxPool(ctrl)
	store := receipt_store.NewMockReceiptStore(ctrl)

	hash1 := types.Hash{1}
	hash2 := types.Hash{2}

	pool.EXPECT().Contains(hash1).Return(true)
	pool.EXPECT().Contains(hash2).Return(false)

	server := NewServer(pool, store)
	require.True(server.IsPending(hash1))
	require.False(server.IsPending(hash2))
}

func TestServer_GetReceipt_RequestsReceiptsFromStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := receipt_store.NewMockReceiptStore(ctrl)

	store.EXPECT().GetReceipt(types.Hash{1}).Return(types.Receipt{Success: true}, true)
	store.EXPECT().GetReceipt(types.Hash{2}).Return(types.Receipt{}, false)

	server := NewServer(nil, store)

	t.Run("ReceiptExists", func(t *testing.T) {
		receipt, ok := server.GetReceipt(types.Hash{1})
		require.True(t, ok)
		require.Equal(t, types.Receipt{Success: true}, receipt)
	})

	t.Run("ReceiptDoesNotExist", func(t *testing.T) {
		receipt, ok := server.GetReceipt(types.Hash{2})
		require.False(t, ok)
		require.Equal(t, types.Receipt{}, receipt)
	})
}
