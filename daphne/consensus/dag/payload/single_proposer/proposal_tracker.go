package single_proposer

import (
	"slices"
	"sync"
)

// BlockNumber is a uint32 alias representing a block number.
// TODO: move to a common types package.
type BlockNumber uint32

// ProposalTracker is a thread-safe structure that tracks proposals seen in the
// network. It retains a list of pending proposals which are automatically
// purged after a certain timeout (defined by TurnTimeoutInRounds). At any time
// users of this utility may query whether a certain block is pending at a
// given round.
//
// Attention: the tracker does not keep track of the highest round number. If
// used with a non-monotonic round number, results are unspecified.
//
// All methods of ProposalTracker are thread-safe.
type ProposalTracker struct {
	pendingProposals []proposalTrackerEntry
	mu               sync.Mutex
}

type proposalTrackerEntry struct {
	round Round
	block BlockNumber
}

// RegisterSeenProposal informs the tracker about a fresh observation of a block
// proposal at a given round. This proposal is tracked until it times
// out.
func (t *ProposalTracker) RegisterSeenProposal(
	round Round,
	block BlockNumber,
) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pendingProposals = append(t.pendingProposals, proposalTrackerEntry{
		round: round,
		block: block,
	})
}

// IsPending checks whether a proposal for the given block is pending at the
// given frame height. If the proposal is pending, it returns true, otherwise
// it returns false.
//
// A side effect of this function is that it purges proposals that are out-dated
// according to the given frame height. Thus, users of this function should
// ensure that the frame number is always monotonically increasing.
func (t *ProposalTracker) IsPending(
	round Round,
	block BlockNumber,
) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pendingProposals = slices.DeleteFunc(
		t.pendingProposals,
		func(entry proposalTrackerEntry) bool {
			return entry.round+TurnTimeoutInRounds < round
		},
	)
	for _, entry := range t.pendingProposals {
		if entry.block == block {
			return true
		}
	}
	return false
}
