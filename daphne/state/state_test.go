package state

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestState_Apply_SuccessfullyProcessTransactions(t *testing.T) {
	genesis := map[types.Address]Account{
		1: {Nonce: 0, Balance: 100},
		2: {Nonce: 0, Balance: 50},
	}

	state := New(genesis)

	transactions := []types.Transaction{
		{From: 1, To: 2, Nonce: 0, Value: 10},
		{From: 2, To: 1, Nonce: 0, Value: 5},
	}

	block := state.Apply(transactions)
	// Both transactions should be processed.
	require.Equal(t, 2, len(block.Transactions))
	require.Equal(t, 2, len(block.Receipts))
	// Check the receipts.
	for _, receipt := range block.Receipts {
		require.True(t, receipt.Success)
	}
	// Fetch accounts.
	account1 := state.GetAccount(1)
	account2 := state.GetAccount(2)
	// Check the balances after processing.
	require.Equal(t, types.Coin(100-10+5), account1.Balance)
	require.Equal(t, types.Coin(50+10-5), account2.Balance)
}

func TestState_Apply_ReportNonceMismatch(t *testing.T) {
	genesis := map[types.Address]Account{
		1: {Nonce: 0, Balance: 100},
		2: {Nonce: 0, Balance: 50},
	}

	state := New(genesis)

	transactions := []types.Transaction{
		{From: 1, To: 2, Nonce: 1, Value: 10}, // Nonce mismatch.
	}

	block := state.Apply(transactions)
	// No transactions should be processed due to nonce mismatch.
	require.Empty(t, block.Transactions)
	require.Empty(t, block.Receipts)
	// Balance should remain unchanged.
	account1 := state.GetAccount(1)
	require.Equal(t, types.Coin(100), account1.Balance)
}

func TestState_Apply_ReportInsufficientFunds(t *testing.T) {
	genesis := map[types.Address]Account{
		1: {Nonce: 0, Balance: 10},
		2: {Nonce: 0, Balance: 50},
	}

	state := New(genesis)

	transactions := []types.Transaction{
		{From: 1, To: 2, Nonce: 0, Value: 20}, // 1 has insufficient funds.
	}

	block := state.Apply(transactions)
	// The transaction SHOULD be processed, even though it fails.
	require.Equal(t, 1, len(block.Transactions))
	// Check the receipt for the failed transaction.
	require.Equal(t, 1, len(block.Receipts))
	require.False(t, block.Receipts[0].Success)
	// Balance should remain unchanged.
	account1 := state.GetAccount(1)
	require.Equal(t, types.Coin(10), account1.Balance)
}

func TestState_GetCurrentBlockNumber(t *testing.T) {
	genesis := map[types.Address]Account{
		1: {Nonce: 0, Balance: 100},
		2: {Nonce: 0, Balance: 100},
	}

	state := New(genesis)
	// Initially, the block number should be 0.
	require.Equal(t, uint32(0), state.GetCurrentBlockNumber())

	// Apply some transactions to increment the block number.
	transactions := []types.Transaction{
		{From: 1, To: 2, Nonce: 0, Value: 10},
		{From: 2, To: 1, Nonce: 0, Value: 5},
		{From: 1, To: 2, Nonce: 1, Value: 5},
		{From: 2, To: 1, Nonce: 1, Value: 10},
	}
	_ = state.Apply(transactions)
	require.Equal(t, uint32(1), state.GetCurrentBlockNumber())
}

func TestState_String(t *testing.T) {
	genesis := map[types.Address]Account{
		1: {Nonce: 0, Balance: 100},
		2: {Nonce: 0, Balance: 100},
	}
	expectedAccounts := []string{
		"Address: #1, Nonce: 0, Balance: 100",
		"Address: #2, Nonce: 0, Balance: 100",
	}
	expectedHeader := "Blockchain State at Block 0:"
	// This cast is safe because we are testing the concrete implementation.
	state := New(genesis)
	// Check the string representation of the header.
	require.Contains(t, state.String(), expectedHeader)
	// Check the string representation of the accounts.
	for _, account := range expectedAccounts {
		require.Contains(t, state.String(), account)
	}
}
