package dekima

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

type LachesisFactory struct{}

// NewLayering creates a new [Lachesis] layering instance.
func (f LachesisFactory) NewLayering(
	dag model.Dag,
	committee *consensus.Committee,
) layering.Layering {
	return newLachesis(dag, committee)
}

func (f LachesisFactory) String() string {
	return "lachesis"
}

type Lachesis struct {
	*Moira
}

func newLachesis(dag model.Dag, committee *consensus.Committee) *Lachesis {
	return &Lachesis{
		Moira: NewMoira(
			&Factory{
				CandidateLayerRelation: dag.StronglyReaches,
				VotingLayerRelation:    dag.StronglyReaches,
				ConsensusLayerRelation: dag.StronglyReaches,
			},
			dag,
			committee,
		),
	}
}
