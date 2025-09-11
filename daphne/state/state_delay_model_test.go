package state

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestFixedProcessingDelayModel_SetBaseTransactionDelay_CorrectlySetsBaseTransactionDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedProcessingDelayModel()

	tx := types.Transaction{
		From:  1,
		To:    2,
		Value: 100,
		Nonce: 1,
	}

	delay := model.GetTransactionDelay(tx)
	require.Equal(0*time.Millisecond, delay)

	model.SetBaseTransactionDelay(100 * time.Millisecond)
	delay = model.GetTransactionDelay(tx)
	require.Equal(100*time.Millisecond, delay)

	model.SetBaseTransactionDelay(250 * time.Millisecond)
	delay = model.GetTransactionDelay(tx)
	require.Equal(250*time.Millisecond, delay)
}

func TestFixedProcessingDelayModel_SetConnectionTransactionDelay_CorrectlySetsCustomTransactionDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedProcessingDelayModel()
	model.SetBaseTransactionDelay(100 * time.Millisecond)

	txs := []types.Transaction{
		{From: 1, To: 2, Value: 100, Nonce: 1},
		{From: 2, To: 1, Value: 50, Nonce: 1},
	}

	delay := model.GetTransactionDelay(txs[0])
	require.Equal(100*time.Millisecond, delay)

	model.SetConnectionTransactionDelay(1, 2, 200*time.Millisecond)
	delay = model.GetTransactionDelay(txs[0])
	require.Equal(200*time.Millisecond, delay)

	delay = model.GetTransactionDelay(txs[1])
	require.Equal(100*time.Millisecond, delay)
}

func TestFixedProcessingDelayModel_SetBaseFinalizationDelay_CorrectlySetsBaseFinalizationDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedProcessingDelayModel()

	delay := model.GetBlockFinalizationDelay(0)
	require.Equal(0*time.Millisecond, delay)

	model.SetBaseFinalizationDelay(100 * time.Millisecond)
	delay = model.GetBlockFinalizationDelay(0)
	require.Equal(100*time.Millisecond, delay)
	delay = model.GetBlockFinalizationDelay(22)
	require.Equal(100*time.Millisecond, delay)

	model.SetBaseFinalizationDelay(150 * time.Millisecond)
	delay = model.GetBlockFinalizationDelay(0)
	require.Equal(150*time.Millisecond, delay)
	delay = model.GetBlockFinalizationDelay(22)
	require.Equal(150*time.Millisecond, delay)
}

func TestFixedProcessingDelayModel_SetCustomFinalizationDelay_CorrectlySetsCustomFinalizationDelay(t *testing.T) {
	require := require.New(t)
	model := NewFixedProcessingDelayModel()

	delay := model.GetBlockFinalizationDelay(0)
	require.Equal(0*time.Millisecond, delay)

	model.SetBaseFinalizationDelay(100 * time.Millisecond)
	delay = model.GetBlockFinalizationDelay(0)
	require.Equal(100*time.Millisecond, delay)

	model.SetCustomBlockFinalizationDelay(22, 150*time.Millisecond)
	delay = model.GetBlockFinalizationDelay(0)
	require.Equal(100*time.Millisecond, delay)
	delay = model.GetBlockFinalizationDelay(22)
	require.Equal(150*time.Millisecond, delay)
}
