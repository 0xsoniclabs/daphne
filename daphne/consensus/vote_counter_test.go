package consensus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommittee_NewVoteCounter_CreatesVoteCounterWithCorrectInitialValues(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{1: 1})
	require.NoError(err)

	voteCounter := NewVoteCounter(committee)
	require.NotNil(voteCounter)
	require.Equal(committee, voteCounter.committee)
	require.NotNil(voteCounter.validatorVotes)
	require.Empty(voteCounter.validatorVotes)
	require.Zero(voteCounter.GetVoteSum())
}

func TestVoteCounter_Vote_IgnoresVoteFromNonExistingCommitteeMember(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{0: 1})
	require.NoError(err)

	voteCounter := NewVoteCounter(committee)
	require.NotNil(voteCounter)

	voteCounter.Vote(1)
	require.Zero(voteCounter.GetVoteSum())
}

func TestVoteCounter_Vote_RegistersVotesForValidValidators(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{1: 100, 2: 200})
	require.NoError(err)

	voteCounter := NewVoteCounter(committee)
	require.NotNil(voteCounter)

	voteCounter.Vote(1)
	voteCounter.Vote(2)

	require.ElementsMatch(voteCounter.validatorVotes.ToSlice(), []ValidatorId{1, 2})
	require.Equal(voteCounter.GetVoteSum(), uint32(300))
}

func TestVoteCounter_Vote_IgnoresVotesFromRepeatedValidators(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{1: 100, 2: 200})
	require.NoError(err)

	voteCounter := NewVoteCounter(committee)
	require.NotNil(voteCounter)

	voteCounter.Vote(1)

	require.ElementsMatch(voteCounter.validatorVotes.ToSlice(), []ValidatorId{1})
	require.Equal(voteCounter.GetVoteSum(), uint32(100))

	voteCounter.Vote(1) // repeated vote
	// No change expected
	require.ElementsMatch(voteCounter.validatorVotes.ToSlice(), []ValidatorId{1})
	require.Equal(voteCounter.GetVoteSum(), uint32(100))
}

func TestVoteCounter_IsQuorumReached_ReturnsCorrectQuorumReachedStatus(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{0: 1, 1: 1, 2: 1, 3: 1})
	require.NoError(err)

	tests := map[string]struct {
		validatorVoters []ValidatorId
		want            bool
	}{
		"all validators vote": {
			validatorVoters: []ValidatorId{0, 1, 2, 3},
			want:            true,
		},
		"minimum number of validators vote": {
			validatorVoters: []ValidatorId{0, 1, 2},
			want:            true,
		},
		"minimum number - 1 of validators vote": {
			validatorVoters: []ValidatorId{0, 1},
			want:            false,
		},
		"minimum number - 2 of validators vote": {
			validatorVoters: []ValidatorId{0},
			want:            false,
		},
		"no validators vote": {
			validatorVoters: []ValidatorId{},
			want:            false,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			voteCounter := NewVoteCounter(committee)
			for _, voter := range testCase.validatorVoters {
				voteCounter.Vote(voter)
			}
			require.Equal(testCase.want, voteCounter.IsQuorumReached())
		})
	}
}

func TestVoteCounter_MajorityReached_ReturnsCorrectMajorityReachedStatus(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{0: 100, 1: 100, 2: 200})
	require.NoError(err)

	tests := map[string]struct {
		validatorVoters []ValidatorId
		want            bool
	}{
		"all validators vote": {
			validatorVoters: []ValidatorId{0, 1, 2},
			want:            true,
		},
		"single validator with 50% stake votes": {
			validatorVoters: []ValidatorId{2},
			want:            true,
		},
		"two validators with 50% total stake vote": {
			validatorVoters: []ValidatorId{0, 1},
			want:            true,
		},
		"validator with less than 50% stake votes": {
			validatorVoters: []ValidatorId{0},
			want:            false,
		},
		"no validators vote": {
			validatorVoters: []ValidatorId{},
			want:            false,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			voteCounter := NewVoteCounter(committee)
			for _, voter := range testCase.validatorVoters {
				voteCounter.Vote(voter)
			}
			require.Equal(testCase.want, voteCounter.IsMajorityReached())
		})
	}
}

func TestVoteCounter_MajorityOfOddTotalStake_RequiresToBeMoreThanHalf(t *testing.T) {
	require := require.New(t)
	committee, err := NewCommittee(map[ValidatorId]uint32{0: 2, 1: 1, 2: 2})
	require.NoError(err)

	require.EqualValues(5, committee.TotalStake())

	counter := NewVoteCounter(committee)
	require.EqualValues(0, counter.GetVoteSum())
	require.False(counter.IsMajorityReached())

	counter.Vote(0)
	require.EqualValues(2, counter.GetVoteSum())
	require.False(counter.IsMajorityReached()) // 2 of 5 is not a majority

	counter.Vote(1)
	require.EqualValues(3, counter.GetVoteSum())
	require.True(counter.IsMajorityReached()) // 3 of 5 is a majority

	counter.Vote(2)
	require.EqualValues(5, counter.GetVoteSum())
	require.True(counter.IsMajorityReached()) // 5 of 5 is a majority
}

func TestVoteCounter_HasAtLeastOneHonestVote_ReturnsCorrectStatus(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{0: 300, 1: 200, 2: 100})
	require.NoError(err)

	tests := map[string]struct {
		validatorVoters []ValidatorId
		want            bool
	}{
		"all honest validators vote": {
			validatorVoters: []ValidatorId{0, 1, 2},
			want:            true,
		},
		"some honest validators vote": {
			validatorVoters: []ValidatorId{0, 1},
			want:            true,
		},
		"no honest validators vote": {
			validatorVoters: []ValidatorId{2},
			want:            false,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			voteCounter := NewVoteCounter(committee)
			for _, voter := range testCase.validatorVoters {
				voteCounter.Vote(voter)
			}
			require.Equal(testCase.want, voteCounter.HasAtLeastOneHonestVote())
		})
	}
}

func TestVoteCounter_HasAtLeastOneHonestVote_SingleValidatorReturnsTrue(t *testing.T) {
	require := require.New(t)

	committee, err := NewCommittee(map[ValidatorId]uint32{1: 1})
	require.NoError(err)

	voteCounter := NewVoteCounter(committee)
	require.False(voteCounter.HasAtLeastOneHonestVote())

	voteCounter.Vote(1)
	require.True(voteCounter.HasAtLeastOneHonestVote())
}
