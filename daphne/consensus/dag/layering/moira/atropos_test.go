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
	committee := consensus.NewUniformCommittee(2)
	dag := newMockedDag(t, committee)
	atropos := newAtropos(dag, committee)

	dag.EXPECT().Reaches(gomock.Any(), gomock.Any()).Times(2)
	atropos.CandidateLayerRelation(nil, nil)
	atropos.VotingLayerRelation(nil, nil)
}
