package consensus

import (
	"errors"
	"maps"
	"slices"
)

// Committee is an immutable set of consensus participants with their respective stakes.
type Committee struct {
	validatorStakeMap map[ValidatorId]uint32
	totalStake        uint32
	quorum            uint32
}

// NewCommittee creates a new Committee from the provided Validator -> Stake mapping.
// If the provided map is empty or the total validator stake is zero, an error is returned.
func NewCommittee(validatorStakeMap map[ValidatorId]uint32) (*Committee, error) {
	if len(validatorStakeMap) == 0 {
		return nil, errors.New("no validators in committee")
	}
	sum := uint32(0)
	for _, stake := range validatorStakeMap {
		sum += stake
	}
	if sum == 0 {
		return nil, errors.New("committee has no stake")
	}
	return &Committee{
		validatorStakeMap: validatorStakeMap,
		totalStake:        sum,
		quorum:            sum*2/3 + 1,
	}, nil
}

// NewUniformCommittee creates a new Committee with the specified number of
// validators, each assigned an equal stake of 1. If the number of validators
// is less than 1, a committee with a single validator is created.
func NewUniformCommittee(numValidators int) *Committee {
	validatorStakeMap := make(map[ValidatorId]uint32)
	for i := range max(numValidators, 1) {
		validatorStakeMap[ValidatorId(i)] = 1
	}
	return &Committee{
		validatorStakeMap: validatorStakeMap,
		totalStake:        uint32(len(validatorStakeMap)),
		quorum:            uint32(len(validatorStakeMap))*2/3 + 1,
	}
}

// GetValidatorStake returns the stake of a validator in the committee.
// If the validator is not found, a zero (idempotent stake) is returned.
func (vc *Committee) GetValidatorStake(validatorId ValidatorId) uint32 {
	return vc.validatorStakeMap[validatorId]
}

// Quorum returns the minimum cumulative stake required from the committee to reach consensus decisions.
func (vc *Committee) Quorum() uint32 {
	return vc.quorum
}

// TotalStake returns the cumulative stake of all validators in the committee.
func (vc *Committee) TotalStake() uint32 {
	return vc.totalStake
}

// Validators returns a slice of all validator IDs in the committee.
func (vc *Committee) Validators() []ValidatorId {
	validators := slices.Collect(maps.Keys(vc.validatorStakeMap))
	slices.Sort(validators)
	return validators
}

// GetHighestStakeValidator returns the ValidatorId of the validator with the
// highest stake in the committee. If multiple validators have the same highest
// stake, the one with the lowest ValidatorId is returned.
func (vc *Committee) GetHighestStakeValidator() ValidatorId {
	best := ValidatorId(0)
	highestStake := uint32(0)
	for vid, stake := range vc.validatorStakeMap {
		if stake > highestStake || (stake == highestStake && vid < best) {
			highestStake = stake
			best = vid
		}
	}
	return best
}

type ValidatorAndStake struct {
	ValidatorId ValidatorId
	Stake       uint32
}
