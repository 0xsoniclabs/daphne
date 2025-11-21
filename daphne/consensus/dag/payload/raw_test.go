package payload

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestRawProtocol_ImplementProtocol(t *testing.T) {
	var _ Protocol[Transactions] = RawProtocol{}
}

func TestRawProtocol_IncludesAllCandidatesInPayload(t *testing.T) {
	candidates := []types.Transaction{
		{From: 1},
		{From: 2},
		{From: 3},
	}

	protocol := RawProtocol{}
	for i := range len(candidates) {
		payload := protocol.BuildPayload(candidates[:i])
		require.Equal(t, []types.Transaction(payload), candidates[:i])
	}
}

func TestRawProtocol_PayloadIsCloneOfCandidates(t *testing.T) {
	candidates := []types.Transaction{{From: 1}}

	protocol := RawProtocol{}
	payload := protocol.BuildPayload(candidates)

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

	require.Len(t, bundles, 1)
	require.Equal(t, []types.Transaction{
		{From: 1},
		{From: 2},
		{From: 3},
		{From: 4},
		{From: 5},
	}, bundles[0].Transactions)
}

func TestRawProtocol_String(t *testing.T) {
	protocol := RawProtocol{}
	require.Equal(t, "raw", protocol.String())
}
