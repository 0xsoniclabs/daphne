package consensus

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestCommittee_NewCommittee_ErrorOnEmptyCommittee(t *testing.T) {
	_, err := NewCommittee(nil)
	require.ErrorContains(t, err, "no creators in committee")

	_, err = NewCommittee(map[model.CreatorId]uint32{})
	require.ErrorContains(t, err, "no creators in committee")
}

func TestCommitteeBuilder_Build_ErrorOnZeroStake(t *testing.T) {
	_, err := NewCommittee(map[model.CreatorId]uint32{0: 0})
	require.ErrorContains(t, err, "no stake")
}

func TestCommittee_GetCreatorStake_ReturnsErrorForUnknownCreator(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[model.CreatorId]uint32{0: 1})
	require.NoError(err)

	_, err = committee.GetCreatorStake(1)
	require.ErrorContains(err, "creator not found")
}

func TestCommittee_GetCreatorStake_ReturnsCorrectStake(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[model.CreatorId]uint32{0: 100, 1: 200})
	require.NoError(err)

	stake, err := committee.GetCreatorStake(0)
	require.NoError(err)
	require.Equal(stake, uint32(100))

	stake, err = committee.GetCreatorStake(1)
	require.NoError(err)
	require.Equal(stake, uint32(200))
}

func TestCommittee_Quorum_ReturnsCorrectCommitteeQuorum(t *testing.T) {
	require := require.New(t)

	tests := map[string]struct {
		creatorStakeMap map[model.CreatorId]uint32
		want            uint32
	}{
		"4 creators with 1 stake each": {
			creatorStakeMap: map[model.CreatorId]uint32{0: 1, 1: 1, 2: 1, 3: 1},
			want:            3,
		},
		"2 creators, 1 with zero stake": {
			creatorStakeMap: map[model.CreatorId]uint32{0: 1, 1: 0},
			want:            1,
		},
		"total stake divisible by 3": {
			creatorStakeMap: map[model.CreatorId]uint32{0: 100, 1: 200},
			want:            201,
		},
		"total stake % 3 == 1": {
			creatorStakeMap: map[model.CreatorId]uint32{0: 101, 1: 200},
			want:            201,
		},
		"total stake % 3 == 2": {
			creatorStakeMap: map[model.CreatorId]uint32{0: 102, 1: 200},
			want:            202,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			committee, err := NewCommittee(testCase.creatorStakeMap)
			require.NoError(err)
			require.Equal(testCase.want, committee.Quorum())
		})
	}
}

func TestCommittee_Creators_ReturnsCorrectCreators(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[model.CreatorId]uint32{0: 1, 1: 2, 2: 3, 3: 4})
	require.NoError(err)

	creators := committee.Creators()
	require.Equal(creators, []model.CreatorId{0, 1, 2, 3})
}
