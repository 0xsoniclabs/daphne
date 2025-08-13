package consensus

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestCommitteeBuilder_AddCreator_ExpandsCommitteeMap(t *testing.T) {
	require := require.New(t)
	committeeBuilder := NewCommitteeBuilder()

	committeeBuilder.
		AddCreator(0, 100).
		AddCreator(1, 200)
	require.Len(committeeBuilder.committee, 2)
	stake, found := committeeBuilder.committee[0]
	require.True(found, "expected creator 0 to be found in committee")
	require.Equal(stake, uint32(100))
	stake, found = committeeBuilder.committee[1]
	require.True(found, "expected creator 1 to be found in committee")
	require.Equal(stake, uint32(200))
}

func TestCommitteeBuilder_AddCreator_UpdatesExistingCreatorStake(t *testing.T) {
	require := require.New(t)
	committeeBuilder := NewCommitteeBuilder()

	committeeBuilder.
		AddCreator(0, 100).
		AddCreator(0, 200)
	require.Len(committeeBuilder.committee, 1)
	stake, found := committeeBuilder.committee[0]
	require.True(found, "expected creator 0 to be found in committee")
	require.Equal(stake, uint32(200))
}

func TestCommitteeBuilder_Build_ErrorOnEmptyCommittee(t *testing.T) {
	committeeBuilder := NewCommitteeBuilder()

	_, err := committeeBuilder.Build()
	require.ErrorContains(t, err, "no creators in committee")
}

func TestCommitteeBuilder_Build_ErrorOnZeroStake(t *testing.T) {
	committeeBuilder := NewCommitteeBuilder()

	committeeBuilder.AddCreator(0, 0)
	_, err := committeeBuilder.Build()
	require.ErrorContains(t, err, "no stake")
}

func TestCommittee_GetCreatorStake_ReturnsErrorForUnknownCreator(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommitteeBuilder().AddCreator(0, 100).Build()
	require.NoError(err)

	_, err = committee.GetCreatorStake(1)
	require.ErrorContains(err, "creator not found")
}

func TestCommittee_GetCreatorStake_ReturnsCorrectStake(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommitteeBuilder().
		AddCreator(0, 100).
		AddCreator(1, 200).
		Build()
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
		creatorsStakes []uint32
		want           uint32
	}{
		"4 creators with 1 stake each": {
			creatorsStakes: []uint32{1, 1, 1, 1},
			want:           3,
		},
		"2 creators, 1 with zero stake": {
			creatorsStakes: []uint32{1, 0},
			want:           1,
		},
		"total stake divisible by 3": {
			creatorsStakes: []uint32{100, 200},
			want:           201,
		},
		"total stake % 3 == 1": {
			creatorsStakes: []uint32{101, 200},
			want:           201,
		},
		"total stake % 3 == 2": {
			creatorsStakes: []uint32{102, 200},
			want:           202,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			builder := NewCommitteeBuilder()
			for i, stake := range testCase.creatorsStakes {
				builder = builder.AddCreator(model.CreatorId(i), stake)
			}
			committee, err := builder.Build()
			require.NoError(err)
			require.Equal(testCase.want, committee.Quorum())
		})
	}
}
