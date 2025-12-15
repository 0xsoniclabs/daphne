package single_proposer

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/stretchr/testify/require"
)

func TestSyncState_Join_ComputesTheMaximumForIndividualStateProperties(t *testing.T) {
	for turnA := range Turn(5) {
		for turnB := range Turn(5) {
			for roundA := range Round(5) {
				for roundB := range Round(5) {
					a := SyncState{
						LastSeenProposalTurn:  turnA,
						LastSeenProposalRound: roundA,
					}
					b := SyncState{
						LastSeenProposalTurn:  turnB,
						LastSeenProposalRound: roundB,
					}
					joined := JoinSyncStates(a, b)
					require.Equal(t, max(turnA, turnB), joined.LastSeenProposalTurn)
					require.Equal(t, max(roundA, roundB), joined.LastSeenProposalRound)
				}
			}
		}
	}
}

func TestIsAllowedToPropose_AcceptsValidProposerTurn(t *testing.T) {
	require := require.New(t)

	validator := consensus.ValidatorId(1)
	committee, err := consensus.NewCommittee(
		map[consensus.ValidatorId]uint32{
			validator: 10,
		},
	)
	require.NoError(err)

	last := ProposalSummary{
		Round: Round(12),
		Turn:  Turn(5),
	}
	next := ProposalSummary{
		Round: Round(14),
		Turn:  Turn(6),
	}
	require.True(IsValidTurnProgression(last, next))

	ok, nextTurn, err := IsAllowedToPropose(
		validator,
		committee,
		SyncState{
			LastSeenProposalTurn:  last.Turn,
			LastSeenProposalRound: last.Round,
		},
		next.Round,
	)
	require.NoError(err)
	require.True(ok)
	require.Equal(next.Turn, nextTurn)
}

func TestIsAllowedToPropose_RejectsInvalidProposerTurn(t *testing.T) {
	validatorA := consensus.ValidatorId(1)
	validatorB := consensus.ValidatorId(2)
	committee, err := consensus.NewCommittee(
		map[consensus.ValidatorId]uint32{
			validatorA: 10,
			validatorB: 20,
		},
	)
	require.NoError(t, err)

	validTurn := Turn(5)
	validProposer, err := GetProposer(committee, validTurn)
	require.NoError(t, err)
	invalidProposer := validatorA
	if invalidProposer == validProposer {
		invalidProposer = validatorB
	}

	// search for a future turn that is no longer valid for the proposer
	invalidTurn := validTurn + 1
	for {
		proposer, err := GetProposer(committee, invalidTurn)
		require.NoError(t, err)
		if proposer != validProposer {
			break
		}
		invalidTurn++
	}

	type input struct {
		validator    consensus.ValidatorId
		currentRound Round
	}

	tests := map[string]func(*input){
		"wrong proposer": func(input *input) {
			input.validator = invalidProposer
		},
		"invalid turn progression": func(input *input) {
			// a proposal made too late needs to be rejected
			input.currentRound = Round(invalidTurn)
		},
	}

	for name, corrupt := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			proposalState := SyncState{
				LastSeenProposalTurn:  12,
				LastSeenProposalRound: 62,
			}

			input := input{
				currentRound: 64,
				validator:    validProposer,
			}

			ok, nextTurn, err := IsAllowedToPropose(
				input.validator,
				committee,
				proposalState,
				input.currentRound,
			)
			require.NoError(err)
			require.True(ok)
			require.Equal(nextTurn, proposalState.LastSeenProposalTurn+1)

			corrupt(&input)
			ok, _, err = IsAllowedToPropose(
				input.validator,
				committee,
				proposalState,
				input.currentRound,
			)
			require.NoError(err)
			require.False(ok)
		})
	}
}

func TestIsAllowedToPropose_ReturnsTurnForTheAllowedProposal(t *testing.T) {
	require := require.New(t)
	validator := consensus.ValidatorId(1)
	committee, err := consensus.NewCommittee(
		map[consensus.ValidatorId]uint32{
			validator: 10,
		},
	)
	require.NoError(err)

	for i := range 50 {
		round := Round(i) + 1
		inputState := SyncState{}

		ok, turn, err := IsAllowedToPropose(
			validator,
			committee,
			inputState,
			round,
		)
		require.NoError(err)
		require.True(ok) // < only 1 validator who is always allowed to propose

		// Check that this is indeed a valid turn progression.
		before := ProposalSummary{
			Turn:  inputState.LastSeenProposalTurn,
			Round: inputState.LastSeenProposalRound,
		}
		after := ProposalSummary{
			Turn:  turn,
			Round: round,
		}
		require.True(
			IsValidTurnProgression(before, after),
			"before: %v, after: %v",
			before, after,
		)
	}
}

func TestIsAllowedToPropose_ForwardsTurnSelectionError(t *testing.T) {
	committee := &consensus.Committee{}

	_, want := GetProposer(committee, Turn(0))
	require.Error(t, want)

	_, _, got := IsAllowedToPropose(
		consensus.ValidatorId(0),
		committee,
		SyncState{},
		Round(0),
	)
	require.Error(t, got)
	require.Equal(t, got, want)
}

func TestGetCurrentTurn_ForKnownExamples_ProducesCorrectTurn(t *testing.T) {
	tests := map[string]struct {
		lastTurn     Turn
		lastRound    Round
		currentRound Round
		want         Turn
	}{
		"same frame": {
			lastTurn:     4,
			lastRound:    5,
			currentRound: 5,
			want:         4,
		},
		"previous frame should not decrease the turn": {
			lastTurn:     4,
			lastRound:    5,
			currentRound: 4,
			want:         4,
		},
		"next frame should not increase the turn": {
			lastTurn:     4,
			lastRound:    5,
			currentRound: 6,
			want:         4,
		},
		"at the time-out frame, it is still the old turn": {
			lastTurn:     4,
			lastRound:    5,
			currentRound: 5 + TurnTimeoutInRounds,
			want:         4,
		},
		"one frame after the timeout it is a new turn": {
			lastTurn:     4,
			lastRound:    5,
			currentRound: 5 + TurnTimeoutInRounds + 1,
			want:         5,
		},
		"multiple timeouts should increase the turn": {
			lastTurn:     4,
			lastRound:    5,
			currentRound: 5 + 2*TurnTimeoutInRounds + 1,
			want:         6,
		},
		"multiple timeouts should increase the turn (2)": {
			lastTurn:     4,
			lastRound:    5,
			currentRound: 5 + 3*TurnTimeoutInRounds + 1,
			want:         7,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := getCurrentTurn(
				SyncState{
					LastSeenProposalTurn:  test.lastTurn,
					LastSeenProposalRound: test.lastRound,
				},
				test.currentRound,
			)
			require.Equal(t, test.want, got)
		})
	}
}

func TestGetCurrentTurn_ForCartesianProductOfInputs_ProducesResultsConsideringTimeouts(t *testing.T) {
	for turn := range Turn(3) {
		for start := range Round(5) {
			for currentRound := range Round(5 * TurnTimeoutInRounds) {
				got := getCurrentTurn(
					SyncState{
						LastSeenProposalTurn:  turn,
						LastSeenProposalRound: start,
					},
					currentRound,
				)

				want := turn
				if currentRound > start {
					delta := currentRound - start - 1
					want += Turn(delta / TurnTimeoutInRounds)
				}
				require.Equal(t, want, got)
			}
		}
	}
}
