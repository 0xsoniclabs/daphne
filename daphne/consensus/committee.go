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

// CommitteeBuilder is a builder for creating a Committee.
type CommitteeBuilder struct {
	committee map[model.CreatorId]uint32
}

// NewCommitteeBuilder creates a new instance of CommitteeBuilder.
func NewCommitteeBuilder() *CommitteeBuilder {
	return &CommitteeBuilder{
		committee: make(map[model.CreatorId]uint32),
	}
}

// AddCreator adds a creator and its stake to the committee.
// If the creator has already been added, the stake is updated.
func (vc *CommitteeBuilder) AddCreator(creatorId model.CreatorId, stake uint32) *CommitteeBuilder {
	vc.committee[creatorId] = stake
	return vc
}

// Build creates a new Committee from the builder.
// If no creators have been added to the builder, an error is returned.
func (vc *CommitteeBuilder) Build() (*Committee, error) {
	if len(vc.committee) == 0 {
		return nil, errors.New("no creators in committee")
	}
	sum := uint32(0)
	for _, stake := range vc.committee {
		sum += stake
	}
	return &Committee{
		creatorStakeMap: vc.committee,
		quorum:          sum*2/3 + 1,
	}, nil
}
