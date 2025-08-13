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
		AddCreator(model.CreatorId(0), 100).
		AddCreator(model.CreatorId(1), 200)
	require.Len(committeeBuilder.committee, 2)
	stake, found := committeeBuilder.committee[model.CreatorId(0)]
	if !found {
		require.Fail("expected creator 0 to be found in committee")
	}
	require.Equal(stake, uint32(100))
	stake, found = committeeBuilder.committee[model.CreatorId(1)]
	if !found {
		require.Fail("expected creator 1 to be found in committee")
	}
	require.Equal(stake, uint32(200))
}

func TestCommitteeBuilder_AddCreator_UpdatesExistingCreatorStake(t *testing.T) {
	require := require.New(t)
	committeeBuilder := NewCommitteeBuilder()

	committeeBuilder.
		AddCreator(model.CreatorId(0), 100).
		AddCreator(model.CreatorId(0), 200)
	require.Len(committeeBuilder.committee, 1)
	stake, found := committeeBuilder.committee[model.CreatorId(0)]
	if !found {
		require.Fail("expected creator 0 to be found in committee")
	}
	require.Equal(stake, uint32(200))
}

func TestCommitteeBuilder_Build_RejectsEmptyCommittee(t *testing.T) {
	committeeBuilder := NewCommitteeBuilder()

	_, err := committeeBuilder.Build()
	require.ErrorContains(t, err, "no creators in committee")
}

func TestCommittee_GetCreatorStake_ReturnsErrorForUnknownCreator(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommitteeBuilder().AddCreator(model.CreatorId(0), 100).Build()
	require.NoError(err)

	_, err = committee.GetCreatorStake(model.CreatorId(1))
	require.ErrorContains(err, "creator not found")
}

func TestCommittee_GetCreatorStake_ReturnsCorrectStake(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommitteeBuilder().
		AddCreator(model.CreatorId(0), 100).
		AddCreator(model.CreatorId(1), 200).
		Build()
	require.NoError(err)

	stake, err := committee.GetCreatorStake(model.CreatorId(0))
	require.NoError(err)
	require.Equal(stake, uint32(100))

	stake, err = committee.GetCreatorStake(model.CreatorId(1))
	require.NoError(err)
	require.Equal(stake, uint32(200))
}

func TestCommittee_Quorum_ReturnsCorrectCommitteeQuorum(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommitteeBuilder().
		AddCreator(model.CreatorId(0), 100).
		AddCreator(model.CreatorId(1), 200).
		Build()
	require.NoError(err)

	require.Equal(committee.Quorum(), uint32(201))

	committee, err = NewCommitteeBuilder().
		AddCreator(model.CreatorId(0), 101).
		AddCreator(model.CreatorId(1), 200).
		Build()
	require.NoError(err)

	require.Equal(committee.Quorum(), uint32(201))
}
