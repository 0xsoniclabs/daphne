package moira

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestLachesisFactory_IsALayeringFactoryImplementation(t *testing.T) {
	var _ layering.Factory = LachesisFactory{}
}

func TestLachesis_IsALayeringImplementation(t *testing.T) {
	var _ layering.Layering = &Lachesis{}
}

func TestLachesisFactory_String_ProducesReadableSummary(t *testing.T) {
	factory := LachesisFactory{}
	require.Equal(t, "lachesis", factory.String())
}

func TestLachesis_NewLayering_SetsRelationsCorrectly(t *testing.T) {
	commitee := consensus.NewUniformCommittee(2)
	dag := newMockedDag(t, commitee)
	lachesis := newLachesis(dag, commitee)

	dag.EXPECT().StronglyReaches(gomock.Any(), gomock.Any()).Times(2)
	lachesis.CandidateLayerRelation(nil, nil)
	lachesis.VotingLayerRelation(nil, nil)
}
