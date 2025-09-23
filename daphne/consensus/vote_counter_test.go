package consensus

import (
	"maps"
	"slices"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestCommittee_NewVoteCounter_CreatesVoteCounterWithCorrectInitialValues(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[model.CreatorId]uint32{1: 1})
	require.NoError(err)

	voteCounter := NewVoteCounter(committee)
	require.NotNil(voteCounter)
	require.Equal(committee, voteCounter.committee)
	require.NotNil(voteCounter.creatorVotes)
	require.Empty(voteCounter.creatorVotes)
	require.Equal(uint32(0), voteCounter.voteSum)
}

func TestVoteCounter_Vote_IgnoresVoteFromNonExistingCommitteeMember(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[model.CreatorId]uint32{0: 1})
	require.NoError(err)

	voteCounter := NewVoteCounter(committee)
	require.NotNil(voteCounter)

	voteCounter.Vote(1)
	require.Zero(voteCounter.voteSum)
}

func TestVoteCounter_Vote_RegistersVotesForValidCreators(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[model.CreatorId]uint32{1: 100, 2: 200})
	require.NoError(err)

	voteCounter := NewVoteCounter(committee)
	require.NotNil(voteCounter)

	voteCounter.Vote(1)
	voteCounter.Vote(2)

	require.ElementsMatch(slices.Collect(maps.Keys(voteCounter.creatorVotes)), []model.CreatorId{1, 2})
	require.Equal(voteCounter.voteSum, uint32(300))
}

func TestVoteCounter_Vote_IgnoresVotesFromRepeatedCreators(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[model.CreatorId]uint32{1: 100, 2: 200})
	require.NoError(err)

	voteCounter := NewVoteCounter(committee)
	require.NotNil(voteCounter)

	voteCounter.Vote(1)

	require.ElementsMatch(slices.Collect(maps.Keys(voteCounter.creatorVotes)), []model.CreatorId{1})
	require.Equal(voteCounter.voteSum, uint32(100))

	voteCounter.Vote(1) // repeated vote
	// No change expected
	require.ElementsMatch(slices.Collect(maps.Keys(voteCounter.creatorVotes)), []model.CreatorId{1})
	require.Equal(voteCounter.voteSum, uint32(100))
}

func TestVoteCounter_IsQuorumReached_ReturnsCorrectQuorumReachedStatus(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[model.CreatorId]uint32{0: 1, 1: 1, 2: 1, 3: 1})
	require.NoError(err)

	tests := map[string]struct {
		creatorVoters []model.CreatorId
		want          bool
	}{
		"all creators vote": {
			creatorVoters: []model.CreatorId{0, 1, 2, 3},
			want:          true,
		},
		"minimum number of creators vote": {
			creatorVoters: []model.CreatorId{0, 1, 2},
			want:          true,
		},
		"minimum number - 1 of creators vote": {
			creatorVoters: []model.CreatorId{0, 1},
			want:          false,
		},
		"minimum number - 2 of creators vote": {
			creatorVoters: []model.CreatorId{0},
			want:          false,
		},
		"no creators vote": {
			creatorVoters: []model.CreatorId{},
			want:          false,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			voteCounter := NewVoteCounter(committee)
			for _, voter := range testCase.creatorVoters {
				voteCounter.Vote(voter)
			}
			require.Equal(testCase.want, voteCounter.IsQuorumReached())
		})
	}
}

func TestVoteCounter_MajorityReached_ReturnsCorrectMajorityReachedStatus(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[model.CreatorId]uint32{0: 100, 1: 100, 2: 200})
	require.NoError(err)

	tests := map[string]struct {
		creatorVoters []model.CreatorId
		want          bool
	}{
		"all creators vote": {
			creatorVoters: []model.CreatorId{0, 1, 2},
			want:          true,
		},
		"single creator with 50% stake votes": {
			creatorVoters: []model.CreatorId{2},
			want:          true,
		},
		"two creators with 50% total stake vote": {
			creatorVoters: []model.CreatorId{0, 1},
			want:          true,
		},
		"creator with less than 50% stake votes": {
			creatorVoters: []model.CreatorId{0},
			want:          false,
		},
		"no creators vote": {
			creatorVoters: []model.CreatorId{},
			want:          false,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			voteCounter := NewVoteCounter(committee)
			for _, voter := range testCase.creatorVoters {
				voteCounter.Vote(voter)
			}
			require.Equal(testCase.want, voteCounter.IsMajorityReached())
		})
	}
}
