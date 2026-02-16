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

package payload

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestDistributedProtocol_BuildPayload_IfOnlyValidator_IncludesEverything(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(1)
	protocol := DistributedProtocol{
		committee:            committee,
		highestNonceProposed: map[types.Address]types.Nonce{},
	}

	txs := []types.Transaction{
		{From: 1, Nonce: 1},
		{From: 2, Nonce: 1},
		{From: 1, Nonce: 2},
	}
	lineup := txpool.NewLineup(txs)

	require.ElementsMatch(txs, protocol.BuildPayload(EventMeta{}, lineup))
}

func TestDistributedProtocol_BuildPayload_MultipleValidators_PartitionTransactions(t *testing.T) {
	const NumTransactions = 100
	const MaxNumValidators = 5
	require := require.New(t)

	info := EventMeta{ParentsMaxRound: 12}
	txs := []types.Transaction{}
	for from := range types.Address(NumTransactions) {
		txs = append(txs, types.Transaction{From: from})
	}
	lineup := txpool.NewLineup(txs)

	// For a varying number of validators, make sure that the transactions are
	// partitioned among them without overlaps or omissions.
	for numValidators := 1; numValidators <= MaxNumValidators; numValidators++ {
		committee := consensus.NewUniformCommittee(numValidators)

		protocols := []DistributedProtocol{}
		for vid := range consensus.ValidatorId(numValidators) {
			protocols = append(protocols, DistributedProtocol{
				committee:            committee,
				localValidatorId:     vid,
				highestNonceProposed: map[types.Address]types.Nonce{},
			})
		}

		// Collect the transactions selected by each validator.
		selected := [][]types.Transaction{}
		for _, protocol := range protocols {
			selected = append(selected, protocol.BuildPayload(info, lineup))
		}

		// Make sure the selection is a partition of the original list.
		merged := []types.Transaction{}
		for _, part := range selected {
			merged = append(merged, part...)
		}
		require.ElementsMatch(txs, merged)
	}
}

func TestDistributedProtocol_BuildPayload_DoesNotReissueTheSameTransactions(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(1)
	protocol := DistributedProtocol{
		committee:            committee,
		highestNonceProposed: map[types.Address]types.Nonce{},
	}

	txs := []types.Transaction{
		{From: 1, Nonce: 1},
		{From: 2, Nonce: 1},
		{From: 1, Nonce: 2},
	}
	lineup := txpool.NewLineup(txs)

	// The first time, everything is accepted.
	require.ElementsMatch(txs, protocol.BuildPayload(EventMeta{}, lineup))

	// Subsequent attempts do not re-emit any of the same transactions.
	require.Empty(protocol.BuildPayload(EventMeta{}, lineup))
	require.Empty(protocol.BuildPayload(EventMeta{}, lineup))

	// Partially new lineups are accepted only for the new transactions.
	lineup2 := txpool.NewLineup([]types.Transaction{
		{From: 1, Nonce: 1}, // already proposed
		{From: 2, Nonce: 2}, // new
	})

	// Only the new transaction is accepted.
	payload := protocol.BuildPayload(EventMeta{}, lineup2)
	require.ElementsMatch([]types.Transaction{
		{From: 2, Nonce: 2},
	}, payload)
}

func TestDistributedProtocol_MergesPayloadsByConcatenationAndSorting(t *testing.T) {
	payloads := []Transactions{
		{{From: 1}, {From: 2}},
		{{From: 3}},
		{{From: 4}, {From: 5}},
	}

	protocol := DistributedProtocol{}
	bundles := protocol.Merge(payloads)

	want := sortTransactionsInExecutionOrder([]types.Transaction{
		{From: 1}, {From: 2}, {From: 3}, {From: 4}, {From: 5},
	})

	require.Len(t, bundles, 1)
	require.Equal(t, want, bundles[0].Transactions)
}

func TestDistributedProtocol_isMyResponsibility_ResponsibilityIsStableForNumberOfRounds(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(3)
	protocol := DistributedProtocol{
		committee:        committee,
		localValidatorId: 1,
	}

	// Choose a nonce and two senders such that only one of them is
	// assigned to validator 1 for the given round.
	round := uint32(100)
	nonce := types.Nonce(288)
	senderA := types.Address(21)
	senderB := types.Address(42)

	require.True(protocol.isMyResponsibility(round, senderA, nonce))
	require.False(protocol.isMyResponsibility(round, senderB, nonce))

	// Verify that the responsibility remains the same for a number of rounds
	// until the next reassignment.
	begin := (round / NumRoundsBetweenReassignments) * NumRoundsBetweenReassignments
	end := begin + NumRoundsBetweenReassignments
	for r := begin; r < end; r++ {
		require.True(protocol.isMyResponsibility(r, senderA, nonce))
		require.False(protocol.isMyResponsibility(r, senderB, nonce))
	}

	// At the next reassignment, the responsibility indeed invert (for these
	// chosen sender/nonce pairs).
	require.False(protocol.isMyResponsibility(end, senderA, nonce))
	require.True(protocol.isMyResponsibility(end, senderB, nonce))
}

func TestDistributedProtocol_isMyResponsibility_ResponsibilityIsStableForNumberOfNonces(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(3)
	protocol := DistributedProtocol{
		committee:        committee,
		localValidatorId: 1,
	}

	sender := types.Address(21)

	// Nonces are assigned to validators in groups of NumNoncesPerPartition.
	for partition := 0; partition < 3; partition++ {
		begin := types.Nonce(partition * NumNoncesPerPartition)
		end := begin + NumNoncesPerPartition

		// Determine the responsibility for the first nonce in the partition.
		isResponsible := protocol.isMyResponsibility(0, sender, begin)
		for n := begin; n < end; n++ {
			require.Equal(isResponsible, protocol.isMyResponsibility(0, sender, n))
		}
	}
}

func TestDistributedProtocolFactory_CreatesRawProtocol(t *testing.T) {
	factory := DistributedProtocolFactory{}
	protocol := factory.NewProtocol(nil, 0)
	require.IsType(t, DistributedProtocol{}, protocol)
}

func TestDistributedProtocolFactory_String(t *testing.T) {
	protocol := DistributedProtocolFactory{}
	require.Equal(t, "distributed", protocol.String())
}
