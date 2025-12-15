package single_proposer

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
)

// The code in this file is a copy of the sync state logic from Sonic's
// single proposer implementation, adapted to fit into Daphne's codebase.
// The original implementation can be found [here]. The main adaptation are:
//  - Renaming "Frame" to "Round" to fit Daphne's terminology.
//  - Epoch handling was removed as Daphne does not support epoch changes.
//
// [here]: https://github.com/0xsoniclabs/sonic/blob/main/inter/proposal_sync_state.go

// SyncState is a structure holding a summary of the state tracked by
// events on the DAG to facilitate the proposal selection.
type SyncState struct {
	LastSeenProposalTurn  Turn
	LastSeenProposalRound Round
}

// JoinSyncStates merges two sync states by taking the maximum of each
// individual field. This is used to aggregate the proposal state from an
// event's parents.
func JoinSyncStates(a, b SyncState) SyncState {
	return SyncState{
		LastSeenProposalTurn:  max(a.LastSeenProposalTurn, b.LastSeenProposalTurn),
		LastSeenProposalRound: max(a.LastSeenProposalRound, b.LastSeenProposalRound),
	}
}

// --- determination of the proposal turn ---

// IsAllowedToPropose checks whether the current validator is allowed to
// propose a new block. If so, the turn for the allowed proposal is returned.
func IsAllowedToPropose(
	validator consensus.ValidatorId,
	validators *consensus.Committee,
	syncState SyncState,
	currentRound Round,
) (bool, Turn, error) {
	// Check whether it is this emitter's turn to propose a new block.
	nextTurn := getCurrentTurn(syncState, currentRound) + 1
	proposer, err := GetProposer(validators, nextTurn)
	if err != nil || proposer != validator {
		return false, 0, err
	}

	// Check that enough time has passed for the next proposal.
	return IsValidTurnProgression(
		ProposalSummary{
			Turn:  syncState.LastSeenProposalTurn,
			Round: syncState.LastSeenProposalRound,
		},
		ProposalSummary{
			Turn:  nextTurn,
			Round: currentRound,
		},
	), nextTurn, nil
}

// getCurrentTurn calculates the current turn based on the last seen proposal
// state and the current frame. This function considers the implicit turn
// progression that occurs if no proposals are made within the timeout period.
func getCurrentTurn(
	syncState SyncState,
	currentRound Round,
) Turn {
	if currentRound <= syncState.LastSeenProposalRound {
		return syncState.LastSeenProposalTurn
	}
	delta := currentRound - syncState.LastSeenProposalRound - 1
	return syncState.LastSeenProposalTurn + Turn(delta/TurnTimeoutInRounds)
}
