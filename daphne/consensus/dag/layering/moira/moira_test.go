package moira

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var _ layering.Factory = Factory{}

func TestFactory_String_ProducesReadableSummary(t *testing.T) {
	factory := Factory{}
	require.Equal(t, "moira", factory.String())
}

func TestMoira_IsALayeringImplementation(t *testing.T) {
	var _ layering.Layering = &Moira{}
}

func TestMoira_IsCandidate_ReturnsFalseForIllegalEvents(t *testing.T) {
	require := require.New(t)

	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{1: 1})
	require.NoError(err)

	lachesis := (&Factory{}).NewLayering(model.NewDag(committee), committee)
	require.False(lachesis.IsCandidate(nil))

	event, err := model.NewEvent(2, nil, nil)
	require.NoError(err)
	require.False(lachesis.IsCandidate(event))
}

func TestMoira_quorumOfRelations_OddTotalStake(t *testing.T) {
	committee := consensus.NewUniformCommittee(11)

	testMoira_quorumOfRelations(t, committee)
}

func TestMoira_quorumOfRelations_EvenTotalStake(t *testing.T) {
	committee := consensus.NewUniformCommittee(12)

	testMoira_quorumOfRelations(t, committee)
}

func testMoira_quorumOfRelations(t *testing.T, committee *consensus.Committee) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	dag := model.NewMockDag(ctrl)
	moira := NewMoira(&Factory{}, dag, committee)

	source, err := model.NewEvent(1, nil, nil)
	require.NoError(err)

	targets := make([]*model.Event, 0, len(committee.Validators()))
	for i := 1; i <= len(committee.Validators()); i++ {
		e, err := model.NewEvent(consensus.ValidatorId(i), nil, nil)
		require.NoError(err)
		targets = append(targets, e)
	}

	for numStronglyReachedEvents := 0; numStronglyReachedEvents <= len(targets); numStronglyReachedEvents++ {
		t.Run(fmt.Sprint("number of strongly reached bases: ", numStronglyReachedEvents), func(t *testing.T) {
			// Use StronglyReaches relation for testing quorumOfRelations.
			for idx, target := range targets {
				dag.EXPECT().StronglyReaches(source, target).Return(idx < numStronglyReachedEvents)
			}

			expected := numStronglyReachedEvents >= len(targets)*2/3+1
			require.Equal(expected, moira.quorumOfRelations(source, targets, dag.StronglyReaches))
		})
	}
}
