package dekima

// import (
// 	"fmt"
// 	"testing"

// 	"github.com/0xsoniclabs/daphne/daphne/consensus"
// 	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/lachesis/db"
// 	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
// 	"github.com/stretchr/testify/require"
// )

// func TestBVC_SonicEventDB_RegularEpoch(t *testing.T) {
// 	// Data representing a usual Sonic epoch with a full-ish validator set.
// 	// Characterized by regular emissions and dense event graph.
// 	testBVC_SonicEventDB_ElectsCorrectLeaders(t, "testdata/events-8000.db", 8000)
// }

// func TestBVC_SonicEventDB_SparseEpoch(t *testing.T) {
// 	// A sparse epoch with fewer validators and irregular emissions.
// 	// Characterized by high frequency of out of order frame elections.
// 	testBVC_SonicEventDB_ElectsCorrectLeaders(t, "testdata/events-1442.db", 1442)
// }

// func testBVC_SonicEventDB_ElectsCorrectLeaders(t *testing.T, dbPath string, epoch int) {
// 	require := require.New(t)

// 	reader, err := db.NewEventDBReader(dbPath)
// 	require.NoError(err)

// 	validators, weights, err := reader.GetValidators(epoch)
// 	require.NoError(err)

// 	validatorStakeMap := map[consensus.ValidatorId]uint32{}
// 	for i, v := range validators {
// 		validatorStakeMap[v] = weights[i]
// 	}

// 	committee, err := consensus.NewCommittee(validatorStakeMap)
// 	require.NoError(err)

// 	dag := model.NewDag(committee)
// 	bvc := NewDekima(GetDefaultFactoryOptions(dag), dag, committee)
// 	electedLeaders := []*model.Event{}

// 	// A map to keep track of Sonic DB events to the corresponding DAG events,
// 	// used for parent resolution.
// 	sonicEventMap := map[*db.DBEvent]*model.Event{}

// 	eventsOrdered, err := reader.GetEvents(epoch)
// 	require.NoError(err)

// 	for _, dbEvent := range eventsOrdered {
// 		// Collect parents. As events are ordered, parents are required to be
// 		// processed already.
// 		parents := []model.EventId{}
// 		for _, parent := range dbEvent.Parents {
// 			parentEvent, exists := sonicEventMap[parent]
// 			require.True(exists, "parent event %s not found for event %+v", parent, dbEvent)

// 			parents = append(parents, parentEvent.EventId())
// 		}
// 		newEvents := dag.AddEvent(model.EventMessage{
// 			Creator: dbEvent.ValidatorId,
// 			Parents: parents,
// 		})
// 		require.Len(newEvents, 1)
// 		// require.Equal(dbEvent.Seq, newEvents[0].Seq())
// 		// require.Equal(dbEvent.Frame, bvc.getEventFrame(newEvents[0]))

// 		// Short imitation of the driver loop for leader election.
// 		leader, _ := bvc.electLeader(bvc.lowestUndecidedFrame)
// 		for leader != nil {
// 			electedLeaders = append(electedLeaders, leader)
// 			leader, _ = bvc.electLeader(bvc.lowestUndecidedFrame)
// 		}
// 		sonicEventMap[dbEvent] = newEvents[0]
// 		// fmt.Println(bvc.getEventFrame(newEvents[0]))
// 	}

// 	sortedLeaders := bvc.SortLeaders(electedLeaders)

// 	electedValidators := []consensus.ValidatorId{}
// 	for _, event := range sortedLeaders {
// 		electedValidators = append(electedValidators, event.Creator())
// 	}

// 	fmt.Println(len(electedValidators))
// }
