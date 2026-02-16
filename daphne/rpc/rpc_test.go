// Copyright 2026 Sonic Labs
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

package rpc

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/receiptstore"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestServer_Send_ForwardTransactionToPoolAndTracksTransactionAsSubmitted(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	tracker := tracker.NewMockTracker(ctrl)
	pool := txpool.NewMockTxPool(ctrl)

	tx := types.Transaction{From: 1}

	tracker.EXPECT().Track(mark.TxSubmitted, "hash", tx.Hash())
	pool.EXPECT().Add(tx).Times(1)

	server := NewServer(pool, nil, tracker)
	require.NoError(server.Send(tx))
}

func TestServer_IsPending_RequestsPresenceOfTransactionInPool(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	pool := txpool.NewMockTxPool(ctrl)

	hash1 := types.Hash{1}
	hash2 := types.Hash{2}

	pool.EXPECT().Contains(hash1).Return(true)
	pool.EXPECT().Contains(hash2).Return(false)

	server := NewServer(pool, nil, nil)
	require.True(server.IsPending(hash1))
	require.False(server.IsPending(hash2))
}

func TestServer_GetReceipt_RequestsReceiptsFromStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := receiptstore.NewMockReceiptStore(ctrl)

	store.EXPECT().GetReceipt(types.Hash{1}).Return(types.Receipt{Success: true}, true)
	store.EXPECT().GetReceipt(types.Hash{2}).Return(types.Receipt{}, false)

	server := NewServer(nil, store, nil)

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
