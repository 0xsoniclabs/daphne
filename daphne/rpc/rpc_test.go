package rpc

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestServer_Send_ForwardTransactionToPool(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	backend := NewMockBackend(ctrl)

	tx := types.Transaction{From: 1}
	backend.EXPECT().Add(tx).Times(1)

	server := NewServer(backend)
	require.NoError(server.Send(tx))
}

func TestServer_IsPending_RequestsPresenceOfTransactionInPool(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	backend := NewMockBackend(ctrl)

	hash1 := types.Hash{1}
	hash2 := types.Hash{2}

	backend.EXPECT().Contains(hash1).Return(true)
	backend.EXPECT().Contains(hash2).Return(false)

	server := NewServer(backend)
	require.True(server.IsPending(hash1))
	require.False(server.IsPending(hash2))
}
