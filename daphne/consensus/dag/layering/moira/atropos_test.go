package moira

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAtroposFactory_IsALayeringFactoryImplementation(t *testing.T) {
	var _ layering.Factory = AtroposFactory{}
}

func TestAtropos_IsALayeringImplementation(t *testing.T) {
	var _ layering.Layering = &Atropos{}
}

func TestAtroposFactory_String_ProducesReadableSummary(t *testing.T) {
	factory := AtroposFactory{}
	require.Equal(t, "atropos", factory.String())
}

func TestAtropos_NewLayering_SetsRelationsCorrectly(t *testing.T) {
	commitee := consensus.NewUniformCommittee(2)
	dag := newMockedDag(t, commitee)
	atropos := newAtropos(dag, commitee)

	dag.EXPECT().Reaches(gomock.Any(), gomock.Any()).Times(2)
	atropos.CandidateLayerRelation(nil, nil)
	atropos.VotingLayerRelation(nil, nil)
}
