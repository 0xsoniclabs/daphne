package emitter

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestPartialSynchronyEmitterFactory_IsAnEmitterFactoryImplementation(t *testing.T) {
	var _ Factory = &PartialSynchronyEmitterFactory{}
}

func TestPartialSynchronyEmitterFactory_String_ProducesReadableSummary(t *testing.T) {
	factory := &PartialSynchronyEmitterFactory{Timeout: 150 * time.Millisecond}
	require.Equal(t, "psynchrony_150ms", factory.String())
}

func TestPartialSynchronyEmitter_IsAnEmitterImplementation(t *testing.T) {
	var _ Emitter = &PartialSynchronyEmitter{}
}

func TestPartialSynchronyEmitter_shouldEmit_alwaysEmitsGenesis(t *testing.T) {
	committee := consensus.NewUniformCommittee(1)
	dag := model.NewDag(committee)
	emitter := &PartialSynchronyEmitter{dag: dag, lastEmittedSeq: 0}

	require.True(t, emitter.shouldEmit(dag.GetHeads()))
}

func TestPartialSynchronyEmitter_observesLatestEmisstion_PreventsDuplicateEmissions(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(1)
	dag := model.NewDag(committee)
	emitter := &PartialSynchronyEmitter{dag: dag, creator: 0}

	// First emission
	require.True(emitter.observesLatestEmisstion(dag.GetHeads()))
	emitter.lastEmittedSeq++

	// Subsequent emission without DAG changes should be prevented
	require.False(emitter.observesLatestEmisstion(dag.GetHeads()))

	events := dag.AddEvent(model.EventMessage{Creator: 0, Parents: []model.EventId{}})
	require.Len(events, 1)

	require.True(emitter.observesLatestEmisstion(dag.GetHeads()))
}

func TestPartialSynchronyEmitter_observesQuorumOfPrevRound_DetectsQuorumCorrectly(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(4)
	dag := model.NewDag(committee)
	emitter := &PartialSynchronyEmitter{dag: dag, creator: 0, committee: committee}

	require.False(emitter.observesQuorumOfPrevRound(dag.GetHeads()))
	dag.AddEvent(model.EventMessage{Creator: 0})
	require.False(emitter.observesQuorumOfPrevRound(dag.GetHeads()))
	emitter.lastEmittedSeq = 1

	dag.AddEvent(model.EventMessage{Creator: 1})
	require.False(emitter.observesQuorumOfPrevRound(dag.GetHeads()))

	dag.AddEvent(model.EventMessage{Creator: 2})
	require.True(emitter.observesQuorumOfPrevRound(dag.GetHeads()))

	dag.AddEvent(model.EventMessage{Creator: 3})
	require.True(emitter.observesQuorumOfPrevRound(dag.GetHeads()))
}

func TestPartialSynchronyEmitter_supportsTheLeader_EvaluatesLeaderSupportCorrectly(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(4)
	dag := model.NewDag(committee)
	emitter := &PartialSynchronyEmitter{dag: dag, committee: committee, leader: 2}

	// Genesis (Round 0) emission
	require.True(emitter.supportsTheLeader(dag.GetHeads()))

	// Round 1 emission
	emitter.lastEmittedSeq = 1

	events := dag.AddEvent(model.EventMessage{Creator: 0})
	require.Len(events, 1)
	e1_0 := events[0]
	require.False(emitter.supportsTheLeader(dag.GetHeads()))

	events = dag.AddEvent(model.EventMessage{Creator: 1})
	e1_1 := events[0]
	require.False(emitter.supportsTheLeader(dag.GetHeads()))

	events = dag.AddEvent(model.EventMessage{Creator: 2})
	require.True(emitter.supportsTheLeader(dag.GetHeads()))
	e1_2 := events[0] // leader

	// Round 2 emission
	emitter.lastEmittedSeq = 2

	dag.AddEvent(model.EventMessage{Creator: 0, Parents: []model.EventId{e1_0.EventId(), e1_2.EventId()}})
	require.False(emitter.supportsTheLeader(dag.GetHeads()))

	dag.AddEvent(model.EventMessage{Creator: 1, Parents: []model.EventId{e1_1.EventId(), e1_2.EventId()}})
	require.False(emitter.supportsTheLeader(dag.GetHeads()))

	dag.AddEvent(model.EventMessage{Creator: 2, Parents: []model.EventId{e1_2.EventId()}})
	require.True(emitter.supportsTheLeader(dag.GetHeads()))
}

func TestPartialSynchronyEmitter_startTimeoutTimer_SetsTimeoutFlagAfterDuration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		require := require.New(t)

		committee := consensus.NewUniformCommittee(1)
		dag := model.NewDag(committee)
		emitter := &PartialSynchronyEmitter{
			dag:     dag,
			creator: 0,
			timeout: 100 * time.Millisecond,
		}

		require.False(emitter.timeoutOccurred.Load())
		emitter.startTimeoutTimer()

		time.Sleep(50 * time.Millisecond)
		require.False(emitter.timeoutOccurred.Load())

		time.Sleep(100 * time.Millisecond)
		require.True(emitter.timeoutOccurred.Load())

		emitter.Stop()
	})
}

func TestPartialSynchronyEmitter_Stop_StopsTimeoutJobAndPreventsAnyFurtherEmissions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		require := require.New(t)
		ctrl := gomock.NewController(t)

		dag := model.NewMockDag(ctrl)
		emitter := &PartialSynchronyEmitter{
			dag:     dag,
			creator: 0,
			timeout: 100 * time.Millisecond,
		}

		emitter.startTimeoutTimer()
		emitter.Stop()

		require.False(emitter.timeoutOccurred.Load())

		time.Sleep(150 * time.Millisecond)
		require.False(emitter.timeoutOccurred.Load())

		// [model.Dag.GetHeads] should not be reached after Stop
		dag.EXPECT().GetHeads().Times(0)
		emitter.OnChange()
	})
}

func TestPartialSynchronyEmitter_PartialSynchronyScenario(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		emitChannel := NewMockChannel(ctrl)

		const primaryLeader, creator = consensus.ValidatorId(1), consensus.ValidatorId(0)
		const timeoutDuration = 500 * time.Millisecond

		committee := consensus.NewUniformCommittee(4)
		dag := model.NewDag(committee)

		// Expect total of 3 emissions, update the DAG accordingly
		emitChannel.EXPECT().Emit(gomock.Any()).Do(func(dagSnapshot map[consensus.ValidatorId]*model.Event) {
			parentIds := make([]model.EventId, 0, len(dagSnapshot))
			if _, found := dagSnapshot[creator]; found {
				parentIds = append(parentIds, dagSnapshot[creator].EventId())
				for creator, tip := range dagSnapshot {
					if creator != tip.Creator() {
						parentIds = append(parentIds, tip.EventId())
					}
				}
			}
			msg := model.EventMessage{Creator: creator, Parents: parentIds}
			dag.AddEvent(msg)
		}).Times(3)

		emitter := (&PartialSynchronyEmitterFactory{
			Committee: committee,
			Timeout:   timeoutDuration,
			Leader:    primaryLeader,
		}).NewEmitter(emitChannel, dag, creator)

		emitter.OnChange() // genesis (1st emission)

		// Attempt another emission without updating the DAG, should not emit
		emitter.OnChange()

		// e_#seq_#validator

		e1_0 := dag.GetHeads()[0]

		// Add another genesis event (primary leader), but should not emit due to lack of quorum
		e1_1 := dag.AddEvent(model.EventMessage{Creator: 1, Parents: []model.EventId{}})[0]
		emitter.OnChange()

		time.Sleep(timeoutDuration / 2)
		// Quorum of round 1 parents observed, primary leader observed + timer NOT triggered (2nd emission)
		e1_2 := dag.AddEvent(model.EventMessage{Creator: 2, Parents: []model.EventId{}})[0]
		emitter.OnChange()

		// Last genesis event, does not trigger any conditions.
		e1_3 := dag.AddEvent(model.EventMessage{Creator: 3, Parents: []model.EventId{}})[0]
		emitter.OnChange()

		// Primary creator event with Seq 2 is added, no conditions met.
		dag.AddEvent(model.EventMessage{Creator: 1, Parents: []model.EventId{e1_1.EventId(), e1_0.EventId(), e1_2.EventId()}})
		emitter.OnChange()

		// Quorum of round 2 parents reached, but no primary leader obsesrved by quorum of them yet.
		dag.AddEvent(model.EventMessage{Creator: 2, Parents: []model.EventId{e1_2.EventId(), e1_3.EventId(), e1_0.EventId()}})

		time.Sleep(timeoutDuration + 100*time.Millisecond)
		// Quorum of round 2 parents reached + timeout triggered and (leader observed by quorum) condition NOT met (3rd emission)

		emitter.Stop()
		time.Sleep(1 * time.Hour)
	})

}
