package single_proposer

// The code in this file is a copy of the turn management logic from Sonic's
// single proposer implementation, adapted to fit into Daphne's codebase.
// The original implementation can be found [here]. The main adaptation are:
//  - Renaming "Frame" to "Round" to fit Daphne's terminology.
//
// [here]: https://github.com/0xsoniclabs/sonic/blob/main/inter/turns.go

// Turn is the turn number of a proposal. Turns are used to orchestrate the
// sequence of block proposals in the consensus protocol. Turns are processed
// in order. A turn ends with a proposer making a proposal or a timeout.
type Turn uint32

// TurnTimeoutInRounds is the number of rounds after which a turn is considered
// failed. Hence, if for the given number of rounds no proposal is made, the
// current turn times out and the next turn is started.
//
// The value is set to 3 rounds after empirical testing of the network has shown
// an average latency of 3 rounds.
const TurnTimeoutInRounds = Round(3)

// IsValidTurnProgression determines whether `next` is a valid successor of
// `last`.
func IsValidTurnProgression(
	last ProposalSummary,
	next ProposalSummary,
) bool {
	// Turns and rounds must strictly increase in each progression step.
	if last.Turn >= next.Turn || last.Round >= next.Round {
		return false
	}

	// Every turn has a window of rounds after the last successful turn during
	// which it is valid to make a proposal. Let l be the round number of the
	// last successful turn t, and q the attempted turn to be made. Then q is
	// valid for the rounds r if
	//
	//                       d * C < r - l <= (d + 1) * C
	// where
	//   - d = q - t - 1 ... number of failed turns between t and q
	//   - C = TurnTimeoutInRounds ... number of rounds after which a turn is
	//     considered to have timed out
	//
	// Thus, the immediate successor turn q = t+1 is valid for the rounds r iff
	//                              0 < r - l <= C
	// which is equivalent to
	//                              l < r <= l + C
	// If the turn t+1 is not materializing, the next turn q = t+2 is valid for
	// the rounds r iff
	//                              C < r - l <= 2 * C
	// which is equivalent to
	//                           l + C < r <= l + 2 * C
	//
	// This rules partition future rounds into intervals of size C, each
	// associated to a specific turn. Thus, for no future round the last seen
	// turn allows more than one proposal to be made. This is important to make
	// sure that validators are not saving up proposals by not using their turns
	// and then proposing all of them in a burst.
	delta := next.Round - last.Round - 1
	return delta/TurnTimeoutInRounds == Round(next.Turn-last.Turn-1)
}

// ProposalSummary is a summary of the metadata of a proposal made in a turn.
type ProposalSummary struct {
	// Round is the round number the proposal was made in.
	Round Round
	// Turn is the turn number the proposal was made in.
	Turn Turn
}
