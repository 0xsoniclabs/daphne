package moira

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

type AtroposFactory struct{}

// NewLayering creates a new [Atropos] layering instance.
func (f AtroposFactory) NewLayering(
	dag model.Dag,
	committee *consensus.Committee,
) layering.Layering {
	return newAtropos(dag, committee)
}

func (f AtroposFactory) String() string {
	return "atropos"
}

type Atropos struct {
	*Moira
}

func newAtropos(dag model.Dag, committee *consensus.Committee) *Atropos {
	return &Atropos{
		Moira: NewMoira(
			&Factory{
				CandidateLayerRelation: dag.StronglyReaches,
				VotingLayerRelation:    dag.StronglyReaches,
			},
			dag,
			committee,
		),
	}
}
