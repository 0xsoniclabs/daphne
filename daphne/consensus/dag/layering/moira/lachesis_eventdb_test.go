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
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/moira/db"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestLachesis_SonicEventDB_RegularEpoch(t *testing.T) {
	// Data representing a usual Sonic epoch with a full-ish validator set.
	// Characterized by regular emissions and dense event graph.
	testLachesis_SonicEventDB_ElectsCorrectLeaders(t, "testdata/events-8000-partial.db", 8000)
}

func TestLachesis_SonicEventDB_SparseEpoch(t *testing.T) {
	// A sparse epoch with fewer validators and irregular emissions.
	// Characterized by high frequency of out of order frame elections.
	testLachesis_SonicEventDB_ElectsCorrectLeaders(t, "testdata/events-1442-partial.db", 1442)
}

func testLachesis_SonicEventDB_ElectsCorrectLeaders(t *testing.T, dbPath string, epoch int) {
	require := require.New(t)

	reader, err := db.NewEventDBReader(dbPath)
	require.NoError(err)

	validators, weights, err := reader.GetValidators(epoch)
	require.NoError(err)

	validatorStakeMap := map[consensus.ValidatorId]uint32{}
	for i, v := range validators {
		validatorStakeMap[v] = weights[i]
	}

	committee, err := consensus.NewCommittee(validatorStakeMap)
	require.NoError(err)

	dag := model.NewDag(committee)
	lachesis := newLachesis(dag, committee)
	electedLeaders := []*model.Event{}

	// A map to keep track of Sonic DB events to the corresponding DAG events,
	// used for parent resolution.
	sonicEventMap := map[*db.DBEvent]*model.Event{}

	eventsOrdered, err := reader.GetEvents(epoch)
	require.NoError(err)

	for _, dbEvent := range eventsOrdered {
		// Collect parents. As events are ordered, parents are required to be
		// processed already.
		parents := []model.EventId{}
		for _, parent := range dbEvent.Parents {
			parentEvent, exists := sonicEventMap[parent]
			require.True(exists, "parent event %s not found for event %+v", parent, dbEvent)

			parents = append(parents, parentEvent.EventId())
		}
		newEvents := dag.AddEvent(model.EventMessage{
			Creator: dbEvent.ValidatorId,
			Parents: parents,
		})
		require.Len(newEvents, 1)
		require.Equal(dbEvent.Seq, newEvents[0].Seq())
		require.Equal(dbEvent.Frame, lachesis.getEventFrame(newEvents[0]))

		// Short imitation of the driver loop for leader election.
		leader, _ := lachesis.electLeader(lachesis.lowestUndecidedFrame)
		for leader != nil {
			electedLeaders = append(electedLeaders, leader)
			leader, _ = lachesis.electLeader(lachesis.lowestUndecidedFrame)
		}
		sonicEventMap[dbEvent] = newEvents[0]
	}

	sortedLeaders := lachesis.SortLeaders(electedLeaders)

	electedValidators := []consensus.ValidatorId{}
	for _, event := range sortedLeaders {
		electedValidators = append(electedValidators, event.Creator())
	}

	expectedValidators, err := reader.GetLeaders(epoch)
	require.NoError(err)

	require.Equal(expectedValidators, electedValidators)
}
