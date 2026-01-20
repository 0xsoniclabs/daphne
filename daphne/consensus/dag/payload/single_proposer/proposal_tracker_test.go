package single_proposer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProposalTracker_RegisterSeenProposal_RegistersProposals(t *testing.T) {
	require := require.New(t)
	tracker := &ProposalTracker{}

	// Initially, no proposals are pending
	require.False(tracker.IsPending(0, 0))
	require.False(tracker.IsPending(0, 1))

	// Register a proposal for block 0 at round 0
	tracker.RegisterSeenProposal(0, 0)
	require.True(tracker.IsPending(0, 0))
	require.False(tracker.IsPending(0, 1))

	// Register a proposal for block 1 at round 1
	tracker.RegisterSeenProposal(1, 1)
	require.True(tracker.IsPending(1, 1))
	require.True(tracker.IsPending(0, 0))
}

func TestProposalTracker_RegisterSeenProposal_CanHandleMultipleProposalsInSameRound(t *testing.T) {
	require := require.New(t)
	tracker := &ProposalTracker{}

	now := Round(5)
	require.False(tracker.IsPending(now, 0))
	require.False(tracker.IsPending(now, 1))

	tracker.RegisterSeenProposal(now, 0)
	tracker.RegisterSeenProposal(now, 1)

	require.True(tracker.IsPending(now, 0))
	require.True(tracker.IsPending(now, 1))

	now++
	require.True(tracker.IsPending(now, 0))
	require.True(tracker.IsPending(now, 1))

	now += TurnTimeoutInRounds - 1
	require.True(tracker.IsPending(now, 0))
	require.True(tracker.IsPending(now, 1))

	now++
	require.False(tracker.IsPending(now, 0))
	require.False(tracker.IsPending(now, 1))
}

func TestProposalTracker_IsPending_InitiallyNoProposalsArePending(t *testing.T) {
	tracker := &ProposalTracker{}
	for b := range BlockNumber(10) {
		for r := range Round(10) {
			require.False(
				t, tracker.IsPending(r, b),
				"block %d should not be pending in round %d", b, r,
			)
		}
	}
}

func TestProposalTracker_IsPending_PurgesOutdatedProposals(t *testing.T) {
	require := require.New(t)
	tracker := &ProposalTracker{}

	block := BlockNumber(123)
	initialRound := Round(12)
	currentRound := initialRound
	require.False(tracker.IsPending(currentRound, block))
	tracker.RegisterSeenProposal(currentRound, block)
	require.True(tracker.IsPending(currentRound, block))

	for range TurnTimeoutInRounds {
		currentRound++
		require.True(tracker.IsPending(currentRound, block))
	}
	require.Equal(initialRound+TurnTimeoutInRounds, currentRound)

	currentRound++
	require.False(tracker.IsPending(currentRound, block))
}
