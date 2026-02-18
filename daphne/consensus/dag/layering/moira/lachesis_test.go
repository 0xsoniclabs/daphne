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
	committee := consensus.NewUniformCommittee(2)
	dag := newMockedDag(t, committee)
	lachesis := newLachesis(dag, committee)

	dag.EXPECT().StronglyReaches(gomock.Any(), gomock.Any()).Times(2)
	lachesis.CandidateLayerRelation(nil, nil)
	lachesis.VotingLayerRelation(nil, nil)
}
