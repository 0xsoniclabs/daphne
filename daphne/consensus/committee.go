package consensus

import (
	"errors"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

// Committee is an immutable set of consensus participants with their respective stakes.
type Committee struct {
	creatorStakeMap map[model.CreatorId]uint32
	quorum          uint32
}

// NewCommittee creates a new Committee from the provided Creator -> Stake mapping.
// If the provided map is empty or the total creator stake is zero, an error is returned.
func NewCommittee(creatorStakeMap map[model.CreatorId]uint32) (*Committee, error) {
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
		quorum:          sum*2/3 + 1,
	}, nil
}

// GetCreatorStake returns the stake of a creator in the committee.
// If the creator is not found, error is returned.
func (vc *Committee) GetCreatorStake(creatorId model.CreatorId) (uint32, error) {
	stake, exists := vc.creatorStakeMap[creatorId]
	if !exists {
		return 0, errors.New("creator not found in committee")
	}
	return stake, nil
}

// Quorum returns the minimum cumulative stake required from the committee to reach consensus decisions.
func (vc *Committee) Quorum() uint32 {
	return vc.quorum
}
