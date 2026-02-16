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

package payload

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestRawProtocol_ImplementProtocol(t *testing.T) {
	var _ Protocol[Transactions] = RawProtocol{}
}

func TestRawProtocol_IncludesAllCandidatesInPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	candidates := []types.Transaction{
		{From: 1},
		{From: 2},
		{From: 3},
	}

	protocol := RawProtocol{}
	for i := range len(candidates) {
		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(candidates[:i])
		payload := protocol.BuildPayload(EventMeta{}, lineup)
		require.Equal(t, []types.Transaction(payload), candidates[:i])
	}
}

func TestRawProtocol_PayloadIsCloneOfCandidates(t *testing.T) {
	ctrl := gomock.NewController(t)
	candidates := []types.Transaction{{From: 1}}

	lineup := txpool.NewMockLineup(ctrl)
	lineup.EXPECT().All().Return(candidates)

	protocol := RawProtocol{}
	payload := protocol.BuildPayload(EventMeta{}, lineup)

	// Modify the original candidates slice
	candidates[0].From = 42

	// Ensure the payload is unaffected
	require.EqualValues(t, 1, payload[0].From)
}

func TestRawProtocol_MergesPayloadsByConcatenation(t *testing.T) {
	payloads := []Transactions{
		{
			{From: 1},
			{From: 2},
		},
		{
			{From: 3},
		},
		{
			{From: 4},
			{From: 5},
		},
	}

	protocol := RawProtocol{}
	bundles := protocol.Merge(payloads)

	want := sortTransactionsInExecutionOrder([]types.Transaction{
		{From: 1},
		{From: 2},
		{From: 3},
		{From: 4},
		{From: 5},
	})

	require.Len(t, bundles, 1)
	require.Equal(t, want, bundles[0].Transactions)
}

func TestRawProtocolFactory_CreatesRawProtocol(t *testing.T) {
	factory := RawProtocolFactory{}
	protocol := factory.NewProtocol(nil, 0)
	require.IsType(t, RawProtocol{}, protocol)
}

func TestRawProtocolFactory_String(t *testing.T) {
	protocol := RawProtocolFactory{}
	require.Equal(t, "raw", protocol.String())
}
