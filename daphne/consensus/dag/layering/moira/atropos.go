package moira

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

// AtroposFactory is used for instantiating [Atropos] which is a specialization of
// the [Moira] layering protocol configured with employing [model.Dag.Reaches]
// as both the CandidateLayerRelation and VotingLayerRelation. This is the weakest
// layering relation which guarantees all the Byzantine Atomic Broadcast properties
// when employed in the [Moira] layering protocol.
// The candidates are layered based on whether they [model.Dag.Reaches] their respective
// lower layers (CandidateLayerRelation), while the layering of voters, as well as
// aggregation rounds use the [model.Dag.StronglyReaches] relation which is enforced
// throughout the Moira protocol.
// The initial voting round (voting layer 0 casting votes for candidates or
// VotingLayerRelation) uses the [model.Dag.Reaches] relation.
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
		Moira: newMoira(
			&Factory{CandidateLayerRelation: dag.Reaches, VotingLayerRelation: dag.Reaches},
			dag,
			committee,
		),
	}
}
