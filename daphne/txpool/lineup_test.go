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

package txpool

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_LineupDecision_String_ReturnsExpectedString(t *testing.T) {
	require.Equal(t, "Accept", LineupAccept.String())
	require.Equal(t, "Reject", LineupReject.String())
}

func Test_WrapLineupFilter_WrapperCallsProvidedFunction(t *testing.T) {
	ctrl := gomock.NewController(t)
	inner := NewMockLineupFilter(ctrl)

	wrapped := WrapLineupFilter(inner.Filter)

	tx := types.Transaction{}
	inner.EXPECT().Filter(tx).Return(LineupAccept)

	decision := wrapped.Filter(tx)
	require.Equal(t, LineupAccept, decision)
}

func Test_lineup_Filter_ProcessesAllTransactionsWhenNoFilterProvided(t *testing.T) {
	tx1 := types.Transaction{Nonce: 1}
	tx2 := types.Transaction{Nonce: 2}
	tx3 := types.Transaction{Nonce: 3}

	txsBySender := map[types.Address][]types.Transaction{
		1: {tx1, tx2},
		2: {tx3},
	}

	lineup := newLineup(txsBySender)
	processed := lineup.Filter(nil)
	require.ElementsMatch(t, []types.Transaction{tx1, tx2, tx3}, processed)
}

func Test_lineup_Filter_FilterIsExposedToAllTransactions(t *testing.T) {
	ctrl := gomock.NewController(t)

	tx1 := types.Transaction{Nonce: 1}
	tx2 := types.Transaction{Nonce: 2}
	tx3 := types.Transaction{Nonce: 3}

	txsBySender := map[types.Address][]types.Transaction{
		1: {tx1, tx2},
		2: {tx3},
	}

	filter := NewMockLineupFilter(ctrl)
	gomock.InOrder(
		filter.EXPECT().Filter(tx1).Return(LineupAccept),
		filter.EXPECT().Filter(tx2).Return(LineupAccept),
	)
	filter.EXPECT().Filter(tx3).Return(LineupAccept)

	lineup := newLineup(txsBySender)
	processed := lineup.Filter(filter)
	require.ElementsMatch(t, []types.Transaction{tx1, tx2, tx3}, processed)
}

func Test_lineup_Filter_RejectingOneTransactionSkipsRemainingTransactionsOfSameSender(t *testing.T) {
	ctrl := gomock.NewController(t)

	tx1 := types.Transaction{Nonce: 1}
	tx2 := types.Transaction{Nonce: 2}
	tx3 := types.Transaction{Nonce: 2}
	tx4 := types.Transaction{Nonce: 3}

	txsBySender := map[types.Address][]types.Transaction{
		1: {tx1, tx2, tx3},
		2: {tx4},
	}

	filter := NewMockLineupFilter(ctrl)
	filter.EXPECT().Filter(tx1).Return(LineupAccept)
	filter.EXPECT().Filter(tx2).Return(LineupReject)
	// tx3 should be skipped
	filter.EXPECT().Filter(tx4).Return(LineupAccept)

	lineup := newLineup(txsBySender)
	processed := lineup.Filter(filter)
	require.ElementsMatch(t, []types.Transaction{tx1, tx4}, processed)
}

func Test_lineup_All_ProducesListOfAllTransactions(t *testing.T) {
	tx1 := types.Transaction{Nonce: 1}
	tx2 := types.Transaction{Nonce: 2}
	tx3 := types.Transaction{Nonce: 3}

	txsBySender := map[types.Address][]types.Transaction{
		1: {tx1, tx2},
		2: {tx3},
	}
	lineup := newLineup(txsBySender)
	allTransactions := lineup.All()
	require.ElementsMatch(t, []types.Transaction{tx1, tx2, tx3}, allTransactions)
}

func Test_newLineup_CreatesLineupWithProvidedTransactions(t *testing.T) {
	txsBySender := map[types.Address][]types.Transaction{
		1: {
			types.Transaction{Nonce: 1},
			types.Transaction{Nonce: 2},
		},
		3: {
			types.Transaction{Nonce: 5},
		},
	}

	lineup := newLineup(txsBySender)
	require.Equal(t, txsBySender, lineup.transactions)
}

func Test_newLineup_FiltersEmptyLists(t *testing.T) {
	input := map[types.Address][]types.Transaction{
		1: {
			types.Transaction{Nonce: 1},
			types.Transaction{Nonce: 2},
		},
		2: {}, // < should be filtered
	}

	want := map[types.Address][]types.Transaction{
		1: {
			types.Transaction{Nonce: 1},
			types.Transaction{Nonce: 2},
		},
	}

	lineup := newLineup(input)
	require.Equal(t, want, lineup.transactions)
}

func Test_newLineup_TruncatesTransactionSequencesWithGapNonces(t *testing.T) {
	input := map[types.Address][]types.Transaction{
		1: {
			types.Transaction{Nonce: 1},
			types.Transaction{Nonce: 2},
			types.Transaction{Nonce: 4}, // < should be filtered
		},
	}

	want := map[types.Address][]types.Transaction{
		1: {
			types.Transaction{Nonce: 1},
			types.Transaction{Nonce: 2},
		},
	}

	lineup := newLineup(input)
	require.Equal(t, want, lineup.transactions)
}

func Test_newLineup_FiltersNonceDuplicates(t *testing.T) {
	input := map[types.Address][]types.Transaction{
		1: {
			types.Transaction{Nonce: 1, To: 10},
			types.Transaction{Nonce: 1, To: 11}, // < should be filtered
		},
	}

	want := map[types.Address][]types.Transaction{
		1: {
			types.Transaction{Nonce: 1, To: 10},
		},
	}

	lineup := newLineup(input)
	require.Equal(t, want, lineup.transactions)
}

func TestNewLineup_CreatesLineupFromTransactions(t *testing.T) {
	txs := []types.Transaction{
		{From: 1, Nonce: 1},
		{From: 1, Nonce: 2},
		{From: 2, Nonce: 1},
	}

	lineup := NewLineup(txs)
	require.ElementsMatch(t, txs, lineup.All())
}

func TestNewLineup_FiltersDuplicates(t *testing.T) {
	txs := []types.Transaction{
		{From: 1, Nonce: 1},
		{From: 1, Nonce: 1},
		{From: 1, Nonce: 2},
		{From: 2, Nonce: 1},
	}

	want := []types.Transaction{
		{From: 1, Nonce: 1},
		{From: 1, Nonce: 2},
		{From: 2, Nonce: 1},
	}

	lineup := NewLineup(txs)
	require.ElementsMatch(t, want, lineup.All())
}

func TestNewLineup_StopsSequencesAtGaps(t *testing.T) {
	txs := []types.Transaction{
		{From: 1, Nonce: 1},
		{From: 1, Nonce: 2},
		{From: 1, Nonce: 4}, // < gap at nonce 2
		{From: 1, Nonce: 5},
		{From: 2, Nonce: 1},
	}

	want := []types.Transaction{
		{From: 1, Nonce: 1},
		{From: 1, Nonce: 2},
		{From: 2, Nonce: 1},
	}

	lineup := NewLineup(txs)
	require.ElementsMatch(t, want, lineup.All())
}
