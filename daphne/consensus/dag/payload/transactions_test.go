package payload

import (
	"bytes"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestTransactions_ImplementPayload(t *testing.T) {
	var _ Payload = Transactions{}
}

func TestTransactions_SizeCalculatesCorrectly(t *testing.T) {
	txs := Transactions{
		{From: 1, To: 2},
		{From: 3, To: 4},
	}

	for i := range len(txs) {
		txsSubset := txs[:i]
		var expectedSize uint32
		for _, tx := range txsSubset {
			expectedSize += tx.MessageSize()
		}
		require.Equal(t, expectedSize, txsSubset.Size())
	}
}

func TestTransactions_CloneCreatesDeepCopy(t *testing.T) {
	original := Transactions{
		{From: 1},
		{From: 2},
	}

	clone := original.Clone().(Transactions)

	// Modify the clone
	clone[0].From = 42

	// Ensure the original is unaffected
	require.EqualValues(t, 1, original[0].From)
}

func Test_sortTransactionsInExecutionOrder_SortsTransactionsByNonce(t *testing.T) {
	txs := []types.Transaction{
		{From: 1, Nonce: 3},
		{From: 2, Nonce: 1},
		{From: 3, Nonce: 2},
	}

	sortedTxs := sortTransactionsInExecutionOrder(txs)

	expectedOrder := []types.Transaction{
		{From: 2, Nonce: 1},
		{From: 3, Nonce: 2},
		{From: 1, Nonce: 3},
	}

	require.Equal(t, expectedOrder, sortedTxs)
}

func Test_sortTransactionsInExecutionOrder_UsesHashAsTieBreaker(t *testing.T) {
	tx1 := types.Transaction{From: 1, Nonce: 1}
	tx2 := types.Transaction{From: 2, Nonce: 1}

	low := tx1
	high := tx2
	hashLow := low.Hash()
	hashHigh := high.Hash()
	if bytes.Compare(hashLow[:], hashHigh[:]) > 0 {
		low, high = high, low
	}

	sorted := sortTransactionsInExecutionOrder([]types.Transaction{tx1, tx2})
	require.Equal(t, []types.Transaction{low, high}, sorted)

	sorted = sortTransactionsInExecutionOrder([]types.Transaction{tx2, tx1})
	require.Equal(t, []types.Transaction{low, high}, sorted)
}
