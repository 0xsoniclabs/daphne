// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

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
