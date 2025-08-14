package consensus

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

// VoteCounter tracks the voting progress of an associated Consensus [Committee].
// It should only be instantiated by calling [NewVoteCounter].
type VoteCounter struct {
	committee    *Committee
	creatorVotes map[model.CreatorId]struct{}
	voteSum      uint32
}

// NewVoteCounter creates a new instance of a [VoteCounter]
// associated with the provided committee.
func NewVoteCounter(vc *Committee) *VoteCounter {
	return &VoteCounter{
		committee:    vc,
		creatorVotes: make(map[model.CreatorId]struct{}),
	}
}

// Vote registers a vote from a provided creator and increments the current
// voting sum by its stake. Repeated votes from the same creator are ignored.
// If the provided creator is not part of the tied committee, error is returned.
func (vc *VoteCounter) Vote(creatorId model.CreatorId) error {
	if _, exists := vc.creatorVotes[creatorId]; exists {
		return nil
	}
	vc.creatorVotes[creatorId] = struct{}{}
	stake, err := vc.committee.GetCreatorStake(creatorId)
	if err != nil {
		return err
	}
	vc.voteSum += stake
	return nil
}

// IsQuorumReached checks if the current voting sum has reached a quorum.
func (vc *VoteCounter) IsQuorumReached() bool {
	return vc.voteSum >= vc.committee.Quorum()
}
