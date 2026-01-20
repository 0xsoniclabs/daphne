package single_proposer

import (
	"testing"
)

func TestIsValidTurnProgression_EnumerationTests(t *testing.T) {
	type S = ProposalSummary
	const C = TurnTimeoutInRounds
	for lastTurn := range Turn(10) {
		for lastRound := range Round(10) {
			for nextTurn := range Turn(C * 10) {
				for nextRound := range Round(C * 10) {
					last := S{
						Turn:  lastTurn,
						Round: lastRound,
					}
					next := S{
						Turn:  nextTurn,
						Round: nextRound,
					}
					want := isValidTurnProgressionForTests(last, next)
					got := IsValidTurnProgression(last, next)
					if want != got {
						t.Errorf("expected %v, got %v", want, got)
					}
				}
			}
		}
	}
}

// isValidTurnProgressionForTests is an alternative implementation of the function to
// compare the results with the production version. It is intended to be a
// reference implementations in tests.
func isValidTurnProgressionForTests(
	last ProposalSummary,
	next ProposalSummary,
) bool {
	// A straightforward implementation of the logic in IsValidTurnProgression
	// explicitly computing the (dC,(d+1)C] intervals.
	if last.Turn >= next.Turn {
		return false
	}
	delta := uint64(next.Turn - last.Turn - 1)
	min := uint64(last.Round) + delta*uint64(TurnTimeoutInRounds)
	max := min + uint64(TurnTimeoutInRounds)
	return min < uint64(next.Round) && uint64(next.Round) <= max
}
