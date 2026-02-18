// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransaction_String_TestTransactionFormatWithFields(t *testing.T) {
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
	tx := Transaction{
		From:  1,
		To:    2,
		Value: 3,
		Nonce: 4,
	}
	t.Run("From", func(t *testing.T) {
		txFromDiff := tx
		txFromDiff.From = 5
		require.NotEqual(t, tx.Hash(), txFromDiff.Hash())
	})
	t.Run("To", func(t *testing.T) {
		txToDiff := tx
		txToDiff.To = 6
		require.NotEqual(t, tx.Hash(), txToDiff.Hash())
	})
	t.Run("Value", func(t *testing.T) {
		txValueDiff := tx
		txValueDiff.Value = 7
		require.NotEqual(t, tx.Hash(), txValueDiff.Hash())
	})
	t.Run("Nonce", func(t *testing.T) {
		txNonceDiff := tx
		txNonceDiff.Nonce = 8
		require.NotEqual(t, tx.Hash(), txNonceDiff.Hash())
	})
}

func TestTransaction_MessageSize_ReturnsCorrectSize(t *testing.T) {
	tx := Transaction{}
	expectedSize := uint32(128) // Expected average size.
	require.Equal(t, expectedSize, tx.MessageSize())
}
