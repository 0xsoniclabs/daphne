package moira

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

// LachesisFactory is used for instantiating [Lachesis] which is a specialization of
// the [Moira] layering protocol configured with [model.Dag.StronglyReaches]
// as both the CandidateLayerRelation and VotingLayerRelation.
// This has the effect that candidates are layered based on whether they strongly
// reach their respective lower layers, where the initial voting round (voting
// layer 0 casting votes for candidates) also uses the [model.Dag.StronglyReaches] relation.
// The layering of voters, as well as aggregation rounds (voting layer > 0),
// uses the default [model.Dag.StronglyReaches] relation again, where in this case this is
// enforced by the Moira protocol itself.
// Such configuration implies that all leaders are also voters for lower frames.
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
		Moira: newMoira(
			&Factory{
				CandidateLayerRelation: dag.StronglyReaches,
				VotingLayerRelation:    dag.StronglyReaches,
			},
			dag,
			committee,
		),
	}
}
