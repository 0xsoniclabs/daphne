package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransaction_String_PrintsWithPrefix(t *testing.T) {
	tests := map[*Transaction]string{
		{}: "Tx{From: #0, To: #0, Value: $0, Nonce: 0}",
		{
			From:  0,
			To:    0,
			Nonce: 0,
			Value: 0,
		}: "Tx{From: #0, To: #0, Value: $0, Nonce: 0}",
		{
			From:  256,
			To:    512,
			Nonce: 1024,
			Value: 2048,
		}: "Tx{From: #256, To: #512, Value: $2048, Nonce: 1024}",
	}

	for tx, expected := range tests {
		t.Run(fmt.Sprintf("%d", tx), func(t *testing.T) {
			require.Equal(t, expected, tx.String())
		})
	}
}

func TestTransaction_Hash_HashUniquenessAffectedByEachField(t *testing.T) {
	tx := &Transaction{
		From:  1,
		To:    2,
		Value: 3,
		Nonce: 4,
	}
	t.Run("From", func(t *testing.T) {
		txFromDiff := *tx
		txFromDiff.From = 5
		require.NotEqual(t, tx.Hash(), txFromDiff.Hash())
	})
	t.Run("To", func(t *testing.T) {
		txToDiff := *tx
		txToDiff.To = 6
		require.NotEqual(t, tx.Hash(), txToDiff.Hash())
	})
	t.Run("Value", func(t *testing.T) {
		txValueDiff := *tx
		txValueDiff.Value = 7
		require.NotEqual(t, tx.Hash(), txValueDiff.Hash())
	})
	t.Run("Nonce", func(t *testing.T) {
		txNonceDiff := *tx
		txNonceDiff.Nonce = 8
		require.NotEqual(t, tx.Hash(), txNonceDiff.Hash())
	})
}
