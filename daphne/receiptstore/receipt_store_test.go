package receiptstore

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestReceiptStore_EmptyStore(t *testing.T) {
	store := NewReceiptStore()
	require.Len(t, store.receipts, 0)
}

func TestReceiptStore_AddBlock_AddsValidBlock(t *testing.T) {
	store := NewReceiptStore()
	tx1 := types.Transaction{Nonce: 1}
	tx2 := types.Transaction{Nonce: 2}
	block := types.Block{
		Transactions: []types.Transaction{tx1, tx2},
		Receipts:     []types.Receipt{{}, {}},
	}

	err := store.AddBlock(block)
	require.NoError(t, err)

	receipt, found := store.GetReceipt(tx1.Hash())
	require.True(t, found)
	require.NotNil(t, receipt)

	receipt, found = store.GetReceipt(tx2.Hash())
	require.True(t, found)
	require.NotNil(t, receipt)
}

func TestReceiptStore_AddBlock_MismatchedTransactionsReceipts(t *testing.T) {
	store := NewReceiptStore()
	tx1 := types.Transaction{
		From:  1,
		To:    2,
		Value: 100,
		Nonce: 1,
	}
	block := types.Block{
		Transactions: []types.Transaction{tx1},
		Receipts:     []types.Receipt{{}, {}}, // Mismatched count
	}

	err := store.AddBlock(block)
	require.ErrorContains(t, err, "mismatched")
}
