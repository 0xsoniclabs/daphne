package consensus

import (
	"fmt"
	"testing"

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
	if !found {
		require.Fail("expected creator 0 to be found in committee")
	}
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
	if !found {
		require.Fail("expected creator 0 to be found in committee")
	}
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

	tests := map[*CommitteeBuilder]uint32{
		NewCommitteeBuilder().
			AddCreator(0, 1).
			AddCreator(1, 1).
			AddCreator(2, 1).
			AddCreator(3, 1): 3,
		NewCommitteeBuilder().
			AddCreator(0, 0).
			AddCreator(1, 1): 1,
		NewCommitteeBuilder().
			AddCreator(0, 100).
			AddCreator(1, 200): 201,
		NewCommitteeBuilder().
			AddCreator(0, 101).
			AddCreator(1, 200): 201,
		NewCommitteeBuilder().
			AddCreator(0, 2).
			AddCreator(1, 101).
			AddCreator(2, 200): 203,
	}

	for commiteeBuilder, expected := range tests {
		t.Run(fmt.Sprintf("%+v", commiteeBuilder.committee), func(t *testing.T) {
			committee, err := commiteeBuilder.Build()
			require.NoError(err)
			require.Equal(expected, committee.Quorum())
		})
	}
}
