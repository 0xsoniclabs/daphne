package consensus

// VoteCounter tracks the voting progress of an associated Consensus [Committee].
// It should only be instantiated by calling [NewVoteCounter].
type VoteCounter struct {
	committee    *Committee
	creatorVotes map[ValidatorId]struct{}
	voteSum      uint32
}

// NewVoteCounter creates a new instance of a [VoteCounter]
// associated with the provided committee.
func NewVoteCounter(vc *Committee) *VoteCounter {
	return &VoteCounter{
		committee:    vc,
		creatorVotes: make(map[ValidatorId]struct{}),
	}
}

// Vote registers a vote from a provided creator and increments the current
// voting sum by its stake. Repeated votes from the same creator are ignored.
// If the provided creator is not part of the associated committee, the vote is ignored.
func (vc *VoteCounter) Vote(creatorId ValidatorId) {
	if _, exists := vc.creatorVotes[creatorId]; exists {
		return
	}
	vc.creatorVotes[creatorId] = struct{}{}
	vc.voteSum += vc.committee.GetCreatorStake(creatorId)
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
