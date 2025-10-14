package consensus

// VoteCounter tracks the voting progress of an associated Consensus [Committee].
// It should only be instantiated by calling [NewVoteCounter].
type VoteCounter struct {
	committee      *Committee
	validatorVotes map[ValidatorId]struct{}
	voteSum        uint32
}

// NewVoteCounter creates a new instance of a [VoteCounter]
// associated with the provided committee.
func NewVoteCounter(vc *Committee) *VoteCounter {
	return &VoteCounter{
		committee:      vc,
		validatorVotes: make(map[ValidatorId]struct{}),
	}
}

// Vote registers a vote from a provided validator and increments the current
// voting sum by its stake. Repeated votes from the same validator are ignored.
// If the provided validator is not part of the associated committee, the vote is ignored.
func (vc *VoteCounter) Vote(validatorId ValidatorId) {
	if _, exists := vc.validatorVotes[validatorId]; exists {
		return
	}
	vc.validatorVotes[validatorId] = struct{}{}
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
