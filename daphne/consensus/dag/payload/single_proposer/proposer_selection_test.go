package single_proposer

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/stretchr/testify/require"
)

func TestGetProposer_IsDeterministic(t *testing.T) {
	require := require.New(t)

	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		1: 10,
		2: 20,
		3: 30,
	})
	require.NoError(err)

	for turn := range Turn(5) {
		a, err := GetProposer(committee, turn)
		require.NoError(err)
		b, err := GetProposer(committee, turn)
		require.NoError(err)
		require.Equal(a, b, "proposer selection is not deterministic")
	}
}

func TestGetProposer_EqualStakes_SelectionIsDeterministic(t *testing.T) {
	require := require.New(t)

	committee1, err := consensus.NewCommittee(
		map[consensus.ValidatorId]uint32{
			1: 10,
			2: 10,
		},
	)
	require.NoError(err)

	committee2, err := consensus.NewCommittee(
		map[consensus.ValidatorId]uint32{
			2: 10,
			1: 10,
		},
	)
	require.NoError(err)

	const N = 50
	want := []consensus.ValidatorId{}
	for turn := range Turn(N) {
		got, err := GetProposer(committee1, turn)
		require.NoError(err)
		want = append(want, got)
	}

	for range 10 {
		counter := 0
		for turn := range Turn(N) {
			got, err := GetProposer(committee2, turn)
			require.NoError(err)
			require.Equal(got, want[counter])
			counter++
		}
	}
}

func TestGetProposer_ZeroStake_IsIgnored(t *testing.T) {
	require := require.New(t)

	committee, err := consensus.NewCommittee(
		map[consensus.ValidatorId]uint32{
			1: 0,
			2: 1,
		},
	)
	require.NoError(err)

	for turn := range Turn(50) {
		a, err := GetProposer(committee, turn)
		require.NoError(err)
		require.Equal(consensus.ValidatorId(2), a, "unexpected proposer")
	}
}

func TestGetProposer_EmptyCommittee_Fails(t *testing.T) {
	require := require.New(t)

	committee := &consensus.Committee{}

	_, err := GetProposer(committee, 0)
	require.ErrorContains(err, "no validators")
}

func TestGetProposer_ProposersAreSelectedProportionalToStake(t *testing.T) {
	t.Parallel()

	validators := map[string][]struct {
		id     consensus.ValidatorId
		weight uint32
	}{
		"single": {
			{id: 1, weight: 10},
		},
		"two-uniform": {
			{id: 1, weight: 10},
			{id: 2, weight: 10},
		},
		"two-biased": {
			{id: 1, weight: 10},
			{id: 2, weight: 20},
		},
		"four-biased": {
			{id: 1, weight: 10},
			{id: 2, weight: 20},
			{id: 3, weight: 30},
			{id: 4, weight: 40},
		},
		"id-gaps": {
			{id: 17, weight: 123},
			{id: 23, weight: 321},
		},
	}

	sizes := []struct {
		samples   int
		tolerance float64 // in percent
	}{
		{samples: 1, tolerance: 100},
		{samples: 10, tolerance: 20},
		{samples: 50, tolerance: 15},
		{samples: 100, tolerance: 10},
		{samples: 1_000, tolerance: 2.5},
		{samples: 10_000, tolerance: 1.6},
		{samples: 100_000, tolerance: 0.3},
	}

	for name, vals := range validators {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stakes := make(map[consensus.ValidatorId]uint32)
			for _, v := range vals {
				stakes[v.id] = v.weight
			}
			validators, err := consensus.NewCommittee(stakes)
			require.NoError(t, err)

			for _, size := range sizes {
				t.Run(fmt.Sprintf("byTurns/samples=%v", size.samples), func(t *testing.T) {
					checkDistribution(
						t, validators, size.samples, size.tolerance,
						func(i int) (consensus.ValidatorId, error) {
							return GetProposer(validators, Turn(i))
						},
					)
				})
			}
		})
	}
}

func checkDistribution(
	t *testing.T,
	committee *consensus.Committee,
	samples int,
	tolerance float64,
	get func(i int) (consensus.ValidatorId, error),
) {
	t.Helper()
	require := require.New(t)
	t.Parallel()
	counters := map[consensus.ValidatorId]int{}
	for i := range samples {
		proposer, err := get(i)
		require.NoError(err)
		require.Contains(committee.Validators(), proposer)
		counters[proposer]++
	}

	tolerance = float64(samples) * tolerance / 100
	total := int(committee.TotalStake())
	for _, id := range committee.Validators() {
		weight := int(committee.GetValidatorStake(id))
		expected := samples * weight / total
		require.InDelta(
			counters[id], expected, tolerance,
			"validator %d is not selected proportional to stake", id,
		)
	}
}
