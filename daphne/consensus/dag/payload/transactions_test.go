package payload

import (
	"testing"

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
