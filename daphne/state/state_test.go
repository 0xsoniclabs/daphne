package state

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestState_Build_WithoutGenesisInstantiatesState(t *testing.T) {
	state := NewStateBuilder().Build()
	require.NotNil(t, state)
}

func TestState_Apply_SuccessfullyProcessTransactions(t *testing.T) {
	genesis := map[types.Address]Account{
		1: {Nonce: 0, Balance: 100},
		2: {Nonce: 0, Balance: 50},
	}

	state := NewState(genesis)

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

	state := NewState(genesis)

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

	state := NewState(genesis)

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

func TestState_Apply_ProducesIncrementingBlockNumbers(t *testing.T) {
	genesis := map[types.Address]Account{
		1: {Nonce: 0, Balance: 100},
		2: {Nonce: 0, Balance: 50},
	}

	state := NewState(genesis)

	transactions := []types.Transaction{
		{From: 1, To: 2, Nonce: 0, Value: 10},
	}

	block1 := state.Apply(transactions)
	require.EqualValues(t, 0, block1.Number)

	block2 := state.Apply(transactions)
	require.EqualValues(t, 1, block2.Number)

	block3 := state.Apply(transactions)
	require.EqualValues(t, 2, block3.Number)
}

func TestState_Apply_TracksTransactionProcessing(t *testing.T) {
	genesis := map[types.Address]Account{
		1: {Nonce: 0, Balance: 100},
		2: {Nonce: 0, Balance: 50},
	}

	transactions := []types.Transaction{
		{From: 1, To: 2, Nonce: 0, Value: 10},
		{From: 2, To: 1, Nonce: 0, Value: 5},
	}

	ctrl := gomock.NewController(t)
	tracker := tracker.NewMockTracker(ctrl)

	gomock.InOrder(
		tracker.EXPECT().Track(mark.TxConfirmed, "hash", transactions[0].Hash(), "block", uint32(0)),
		tracker.EXPECT().Track(mark.TxBeginProcessing, "hash", transactions[0].Hash(), "block", uint32(0)),
		tracker.EXPECT().Track(mark.TxEndProcessing, "hash", transactions[0].Hash(), "block", uint32(0)),
		tracker.EXPECT().Track(mark.TxFinalized, "hash", transactions[0].Hash(), "block", uint32(0)),
	)
	gomock.InOrder(
		tracker.EXPECT().Track(mark.TxConfirmed, "hash", transactions[1].Hash(), "block", uint32(0)),
		tracker.EXPECT().Track(mark.TxBeginProcessing, "hash", transactions[1].Hash(), "block", uint32(0)),
		tracker.EXPECT().Track(mark.TxEndProcessing, "hash", transactions[1].Hash(), "block", uint32(0)),
		tracker.EXPECT().Track(mark.TxFinalized, "hash", transactions[1].Hash(), "block", uint32(0)),
	)

	state := NewStateBuilder().WithGenesis(genesis).WithTracker(tracker).Build()

	state.Apply(transactions)
}

func TestState_GetCurrentBlockNumber(t *testing.T) {
	genesis := map[types.Address]Account{
		1: {Nonce: 0, Balance: 100},
		2: {Nonce: 0, Balance: 100},
	}

	state := NewState(genesis)
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
	state := NewState(genesis)
	// Check the string representation of the header.
	require.Contains(t, state.String(), expectedHeader)
	// Check the string representation of the accounts.
	for _, account := range expectedAccounts {
		require.Contains(t, state.String(), account)
	}
}

func TestState_StateWithDelayModel_EnforcesDelays(t *testing.T) {
	tests := map[string]struct {
		transactionDelay       time.Duration
		blockFinalizationDelay time.Duration
	}{
		"transaction delay only": {
			transactionDelay:       60 * time.Millisecond,
			blockFinalizationDelay: 0 * time.Millisecond,
		},
		"block finalization delay only": {
			transactionDelay:       0 * time.Millisecond,
			blockFinalizationDelay: 50 * time.Millisecond,
		},
		"transaction and block finalization delay": {
			transactionDelay:       60 * time.Millisecond,
			blockFinalizationDelay: 50 * time.Millisecond,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				require := require.New(t)

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				model := NewMockProcessingDelayModel(ctrl)

				g := Genesis{
					1: {Nonce: 0, Balance: 100},
					2: {Nonce: 0, Balance: 100},
				}

				state := NewStateBuilder().WithGenesis(g).WithDelayModel(model).Build()

				transactions := []types.Transaction{
					{From: 1, To: 2, Nonce: 0, Value: 10},
					{From: 2, To: 1, Nonce: 0, Value: 5},
					{From: 2, To: 1, Nonce: 1, Value: 5},
				}

				for _, tx := range transactions {
					model.EXPECT().
						GetTransactionDelay(tx).
						Return(testCase.transactionDelay)
				}
				model.EXPECT().GetBlockFinalizationDelay(
					state.blockNumber,
					transactions,
				).Return(testCase.blockFinalizationDelay)

				start := time.Now()
				state.Apply(transactions)
				elapsed := time.Since(start)

				require.Equal(testCase.transactionDelay*time.Duration(len(transactions))+testCase.blockFinalizationDelay, elapsed)
			})
		})
	}
}

func TestState_Process_TransactionWithWrongNonce_SignalsSkipToTracker(t *testing.T) {
	ctrl := gomock.NewController(t)
	tracker := tracker.NewMockTracker(ctrl)

	state := NewStateBuilder().
		WithGenesis(Genesis{1: {Nonce: 4}}).
		WithTracker(tracker).
		Build()

	tx := types.Transaction{
		From:  1,
		Nonce: 3, // < wrong nonce, should be 4
	}

	tracker.EXPECT().Track(mark.TxSkipped, "hash", tx.Hash(), "block", uint32(12))
	state.process(tx, 12)
}
