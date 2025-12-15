package consensus

import (
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

// VoteCounter tracks the voting progress of an associated Consensus [Committee].
// It should only be instantiated by calling [NewVoteCounter].
type VoteCounter struct {
	committee      *Committee
	validatorVotes sets.Set[ValidatorId]
	voteSum        uint32
}

// NewVoteCounter creates a new instance of a [VoteCounter]
// associated with the provided committee.
func NewVoteCounter(vc *Committee) *VoteCounter {
	return &VoteCounter{
		committee: vc,
	}
}

// Vote registers a vote from a provided validator and increments the current
// voting sum by its stake. Repeated votes from the same validator are ignored.
// If the provided validator is not part of the associated committee, the vote is ignored.
func (vc *VoteCounter) Vote(validatorId ValidatorId) {
	if vc.validatorVotes.Contains(validatorId) {
		return
	}
	vc.validatorVotes.Add(validatorId)
	vc.voteSum += vc.committee.GetValidatorStake(validatorId)
}

// IsQuorumReached checks if the current voting sum has reached a quorum.
func (vc *VoteCounter) IsQuorumReached() bool {
	return vc.voteSum >= vc.committee.Quorum()
}

// IsMajorityReached checks if the current voting sum has reached a simple majority (>=50%).
func (vc *VoteCounter) IsMajorityReached() bool {
	totalStake := vc.committee.TotalStake()
	return vc.voteSum >= totalStake-totalStake/2
}

// HasAtLeastOneHonestVote checks if at least one honest validator has voted.
func (vc *VoteCounter) HasAtLeastOneHonestVote() bool {
	totalStake := vc.committee.TotalStake()
	return vc.voteSum >= totalStake/3+1
}

// GetVoteSum returns the current sum of cast votes.
func (vc *VoteCounter) GetVoteSum() uint32 {
	return vc.voteSum
}

// Clone clones the vote counter.
func (vc *VoteCounter) Clone() *VoteCounter {
	clone := *vc
	clone.validatorVotes = vc.validatorVotes.Clone()
	return &clone
}

// Hash returns a hash of the vote counter.
func (vc *VoteCounter) Hash() types.Hash {
	// Dereferencing matters to avoid differing hashes due to pointer addresses.
	return types.Sha256([]byte(fmt.Sprintf("%+v%+v", vc.validatorVotes, *vc.committee)))
}
