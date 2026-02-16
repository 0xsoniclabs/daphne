// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package consensus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommittee_NewCommittee_ErrorOnEmptyCommittee(t *testing.T) {
	_, err := NewCommittee(nil)
	require.ErrorContains(t, err, "no validators in committee")

	_, err = NewCommittee(map[ValidatorId]uint32{})
	require.ErrorContains(t, err, "no validators in committee")
}

func TestCommittee_NewUniformCommittee_CreatesCommitteeWithAtLeastOneValidator(t *testing.T) {
	require := require.New(t)

	committee := NewUniformCommittee(0)
	require.Equal(1, len(committee.Validators()))

	committee = NewUniformCommittee(-5)
	require.Equal(1, len(committee.Validators()))

	committee = NewUniformCommittee(3)
	require.Equal(3, len(committee.Validators()))
}

func TestCommittee_NewUniformCommittee_EveryValidatorHasStakeOne(t *testing.T) {
	require := require.New(t)

	committee := NewUniformCommittee(5)
	for _, validatorId := range committee.Validators() {
		stake := committee.GetValidatorStake(validatorId)
		require.Equal(uint32(1), stake)
	}
}

func TestCommitteeBuilder_Build_ErrorOnZeroStake(t *testing.T) {
	_, err := NewCommittee(map[ValidatorId]uint32{0: 0})
	require.ErrorContains(t, err, "no stake")
}

func TestCommittee_GetValidatorStake_ReturnsZeroStakeForUnknownValidator(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{0: 1})
	require.NoError(err)

	require.Zero(committee.GetValidatorStake(1))
}

func TestCommittee_GetValidatorStake_ReturnsCorrectStake(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{0: 100, 1: 200})
	require.NoError(err)

	stake := committee.GetValidatorStake(0)
	require.Equal(stake, uint32(100))

	stake = committee.GetValidatorStake(1)
	require.Equal(stake, uint32(200))
}

func TestCommittee_Quorum_ReturnsCorrectCommitteeQuorum(t *testing.T) {
	require := require.New(t)

	tests := map[string]struct {
		validatorStakeMap map[ValidatorId]uint32
		want              uint32
	}{
		"4 validators with 1 stake each": {
			validatorStakeMap: map[ValidatorId]uint32{0: 1, 1: 1, 2: 1, 3: 1},
			want:              3,
		},
		"2 validators, 1 with zero stake": {
			validatorStakeMap: map[ValidatorId]uint32{0: 1, 1: 0},
			want:              1,
		},
		"total stake divisible by 3": {
			validatorStakeMap: map[ValidatorId]uint32{0: 100, 1: 200},
			want:              201,
		},
		"total stake % 3 == 1": {
			validatorStakeMap: map[ValidatorId]uint32{0: 101, 1: 200},
			want:              201,
		},
		"total stake % 3 == 2": {
			validatorStakeMap: map[ValidatorId]uint32{0: 102, 1: 200},
			want:              202,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			committee, err := NewCommittee(testCase.validatorStakeMap)
			require.NoError(err)
			require.Equal(testCase.want, committee.Quorum())
		})
	}
}

func TestCommittee_Validators_ReturnsCorrectValidators(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{0: 1, 1: 2, 2: 3, 3: 4})
	require.NoError(err)

	validators := committee.Validators()
	require.Equal(validators, []ValidatorId{0, 1, 2, 3})
}

func TestCommittee_TotalStake_ReturnsCorrectTotalStake(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{0: 100, 1: 200, 2: 300})
	require.NoError(err)

	totalStake := committee.TotalStake()
	require.Equal(totalStake, uint32(600))
}

func TestCommittee_GetHighestStakeValidator_ReturnsTheValidatorsWithHighestStake(t *testing.T) {
	stakes := map[ValidatorId]uint32{0: 100, 1: 200, 2: 200}
	committee, err := NewCommittee(stakes)
	require.NoError(t, err)
	require.Equal(t, ValidatorId(1), committee.GetHighestStakeValidator())
}

func TestCommittee_GetHighestStakeValidator_EmptyCommittee_ReturnsZero(t *testing.T) {
	committee := &Committee{}
	require.Zero(t, committee.GetHighestStakeValidator())
}
