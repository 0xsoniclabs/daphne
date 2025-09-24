package consensus

import (
	"errors"
	"maps"
	"slices"
)

// Committee is an immutable set of consensus participants with their respective stakes.
type Committee struct {
	creatorStakeMap map[ValidatorId]uint32
	totalStake      uint32
	quorum          uint32
}

// NewCommittee creates a new Committee from the provided Creator -> Stake mapping.
// If the provided map is empty or the total creator stake is zero, an error is returned.
func NewCommittee(creatorStakeMap map[ValidatorId]uint32) (*Committee, error) {
	if len(creatorStakeMap) == 0 {
		return nil, errors.New("no creators in committee")
	}
	sum := uint32(0)
	for _, stake := range creatorStakeMap {
		sum += stake
	}
	if sum == 0 {
		return nil, errors.New("committee has no stake")
	}
	return &Committee{
		creatorStakeMap: creatorStakeMap,
		totalStake:      sum,
		quorum:          sum*2/3 + 1,
	}, nil
}

// GetCreatorStake returns the stake of a creator in the committee.
// If the creator is not found, a zero (idempotent stake) is returned.
func (vc *Committee) GetCreatorStake(creatorId ValidatorId) uint32 {
	return vc.creatorStakeMap[creatorId]
}

// Quorum returns the minimum cumulative stake required from the committee to reach consensus decisions.
func (vc *Committee) Quorum() uint32 {
	return vc.quorum
}

// TotalStake returns the cumulative stake of all creators in the committee.
func (vc *Committee) TotalStake() uint32 {
	return vc.totalStake
}

// Creators returns a slice of all creator IDs in the committee.
func (vc *Committee) Creators() []ValidatorId {
	creators := slices.Collect(maps.Keys(vc.creatorStakeMap))
	slices.Sort(creators)
	return creators
}
