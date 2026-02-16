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
