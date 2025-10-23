package lachesis

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/utils/db"
	"github.com/stretchr/testify/require"
)

func TestLachesis_SonicEventDB_SeqAssignedCorrectly(t *testing.T) {
	require := require.New(t)

	reader, err := db.NewEventDBReader("testdata/events-8000-partial.db")
	require.NoError(err)

	orderedDBEvents, err := reader.GetEvents(8000)
	require.NoError(err)

	dbEventMap := map[*db.DBEvent]*model.Event{}

	for _, dbEvent := range orderedDBEvents[:200] {
		// Collect parents. As events are ordered, parents must have been
		// processed already.
		parents := []*model.Event{}
		for _, parent := range dbEvent.Parents {
			parentEvent, exists := dbEventMap[parent]
			require.True(exists, "parent event %s not found for event %+v", parent, dbEvent)

			parents = append(parents, parentEvent)
		}

		newEvent, err := model.NewEvent(dbEvent.ValidatorId, parents, nil)
		require.NoError(err, "event creation failed for event %+v: %v", dbEvent, err)
		require.Equal(dbEvent.Seq, newEvent.Seq())

		dbEventMap[dbEvent] = newEvent
	}
}

func testLachesis_SonicEventDB_ElectsCorrectLeaders(t *testing.T, dbPath string, epoch int) {
	require := require.New(t)

	reader, err := db.NewEventDBReader(dbPath)
	require.NoError(err)

	eventsOrdered, err := reader.GetEvents(epoch)
	require.NoError(err)

	sonicEventMap := map[*db.DBEvent]*model.Event{}

	validators, weights, err := reader.GetValidators(epoch)
	require.NoError(err)

	validatorStakeMap := map[consensus.ValidatorId]uint32{}
	for i, v := range validators {
		validatorStakeMap[v] = weights[i]
	}

	committee, err := consensus.NewCommittee(validatorStakeMap)
	require.NoError(err)

	dag := model.NewDag()
	lachesis := newLachesis(committee)

	leaders := []*model.Event{}

	for _, dbEvent := range eventsOrdered {
		// Collect parents. As events are ordered, parents must have been
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

		// Fast imitation of the driver loop to elect leaders.
		leader, _ := lachesis.electLeader(dag, lachesis.lowestUndecidedFrame)
		for leader != nil {
			leaders = append(leaders, leader)
			leader, _ = lachesis.electLeader(dag, lachesis.lowestUndecidedFrame)
		}
		sonicEventMap[dbEvent] = newEvents[0]
	}

	sortedLeaders := lachesis.SortLeaders(dag, leaders)
	electedValidators := []consensus.ValidatorId{}
	for _, event := range sortedLeaders {
		electedValidators = append(electedValidators, event.Creator())
	}

	expectedValidators, err := reader.GetLeaders(epoch)
	require.NoError(err)

	require.Equal(expectedValidators, electedValidators)
}

func TestLachesis_SonicEventDB_RegularEpoch(t *testing.T) {
	testLachesis_SonicEventDB_ElectsCorrectLeaders(t, "testdata/events-8000-partial.db", 8000)
}

func TestLachesisOnRegressionData_SparseEpoch(t *testing.T) {
	testLachesis_SonicEventDB_ElectsCorrectLeaders(t, "testdata/events-1442.db", 1442)
}
