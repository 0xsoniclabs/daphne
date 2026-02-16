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

package emitter

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestPartialSynchronyEmitterFactory_IsAnEmitterFactoryImplementation(t *testing.T) {
	var _ Factory = &PartialSynchronyEmitterFactory{}
}

func TestPartialSynchronyEmitterFactory_String_ProducesReadableSummary(t *testing.T) {
	factory := &PartialSynchronyEmitterFactory{TimeoutDuration: 150 * time.Millisecond}
	require.Equal(t, "psynchrony_150ms", factory.String())
}

func TestPartialSynchronyEmitter_IsAnEmitterImplementation(t *testing.T) {
	var _ Emitter = &PartialSynchronyEmitter{}
}

func TestPartialSynchronyEmitter_shouldEmit_alwaysEmitsGenesis(t *testing.T) {
	committee := consensus.NewUniformCommittee(1)
	dag := model.NewDag(committee)
	emitter := &PartialSynchronyEmitter{dag: dag, latestEmittedRound: 0}

	require.True(t, emitter.shouldEmit(dag.GetHeads(), 1))
}

func TestPartialSynchronyEmitter_observesLatestEmission_PreventsDuplicateEmissions(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(1)
	dag := model.NewDag(committee)
	emitter := &PartialSynchronyEmitter{dag: dag, creator: 0}

	// First emission
	require.True(emitter.observesLatestEmission(dag.GetHeads()))
	emitter.latestEmittedRound++

	// Subsequent emission without DAG changes should be prevented
	require.False(emitter.observesLatestEmission(dag.GetHeads()))
	require.False(emitter.shouldEmit(dag.GetHeads(), emitter.latestEmittedRound+1))

	events := dag.AddEvent(model.EventMessage{Creator: 0, Parents: []model.EventId{}})
	require.Len(events, 1)

	require.True(emitter.observesLatestEmission(dag.GetHeads()))
}

func TestPartialSynchronyEmitter_observesQuorumOfPrevRound_DetectsQuorumCorrectly(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	committee := consensus.NewUniformCommittee(4)
	dag := model.NewDag(committee)
	layering := layering.NewMockLayering(ctrl)
	layering.EXPECT().GetRound(gomock.Any()).Return(uint32(1)).AnyTimes()

	emitter := &PartialSynchronyEmitter{dag: dag, creator: 0, committee: committee, layering: layering}

	dag.AddEvent(model.EventMessage{Creator: 0})
	require.False(emitter.observesQuorumOfRound(dag.GetHeads(), 1))

	dag.AddEvent(model.EventMessage{Creator: 1})
	require.False(emitter.observesQuorumOfRound(dag.GetHeads(), 1))

	dag.AddEvent(model.EventMessage{Creator: 2})
	require.True(emitter.observesQuorumOfRound(dag.GetHeads(), 1))

	dag.AddEvent(model.EventMessage{Creator: 3})
	require.True(emitter.observesQuorumOfRound(dag.GetHeads(), 1))
}

func TestPartialSynchronyEmitter_roundToEmit_ChoosesCorrectRoundBasedOnDAG(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	committee := consensus.NewUniformCommittee(4)
	layering := layering.NewMockLayering(ctrl)
	layering.EXPECT().GetRound(gomock.Any()).DoAndReturn(func(event *model.Event) uint32 {
		return event.Seq()
	}).AnyTimes()
	emitter := &PartialSynchronyEmitter{committee: committee, layering: layering}

	// Empty DAG, emit the genesis round (1)
	require.Equal(uint32(1), emitter.roundToEmit(map[consensus.ValidatorId]*model.Event{}))

	// Have received some genesis events from other creators, but none higher
	// Should still emit round 1.
	e1_2, err := model.NewEvent(2, nil, nil, time.Time{})
	require.NoError(err)
	e1_3, err := model.NewEvent(3, nil, nil, time.Time{})
	require.NoError(err)

	require.Equal(uint32(1), emitter.roundToEmit(map[consensus.ValidatorId]*model.Event{
		2: e1_2,
		3: e1_3,
	}))

	// Received a quorum of other genesis events.
	// It is not worth emitting round 1 anymore.
	e1_1, err := model.NewEvent(1, nil, nil, time.Time{})
	require.NoError(err)

	require.Equal(uint32(2), emitter.roundToEmit(map[consensus.ValidatorId]*model.Event{
		1: e1_1,
		2: e1_2,
		3: e1_3,
	}))
}

func TestPartialSynchronyEmitter_selectParents_SelectsCorrectParentsForRound(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(3)
	layering := layering.NewMockLayering(gomock.NewController(t))
	layering.EXPECT().GetRound(gomock.Any()).DoAndReturn(func(event *model.Event) uint32 {
		return event.Seq()
	}).AnyTimes()
	emitter := &PartialSynchronyEmitter{committee: committee, layering: layering}

	require.Empty(emitter.selectParents(map[consensus.ValidatorId]*model.Event{}, 1))

	e1_0, err := model.NewEvent(0, nil, nil, time.Time{})
	require.NoError(err)
	e1_1, err := model.NewEvent(1, nil, nil, time.Time{})
	require.NoError(err)
	e1_2, err := model.NewEvent(2, nil, nil, time.Time{})
	require.NoError(err)

	allParentsInPrevRound := map[consensus.ValidatorId]*model.Event{
		0: e1_0,
		1: e1_1,
		2: e1_2,
	}
	require.Equal(allParentsInPrevRound, emitter.selectParents(allParentsInPrevRound, 2))

	e2_0, err := model.NewEvent(0, []*model.Event{e1_0}, nil, time.Time{})
	require.NoError(err)
	e2_2, err := model.NewEvent(2, []*model.Event{e1_2, e1_0}, nil, time.Time{})
	require.NoError(err)
	e3_2, err := model.NewEvent(2, []*model.Event{e2_2}, nil, time.Time{})
	require.NoError(err)

	// Creator 1 is in round 1, creator 0 in round 2, while creator 2 is in round 3.
	oneParentAheadOneParentBehind := map[consensus.ValidatorId]*model.Event{
		0: e2_0,
		1: e1_1,
		2: e3_2,
	}
	// As the emission is for round 3, pick parents from round 2 (or earlier if not available)
	require.Equal(map[consensus.ValidatorId]*model.Event{
		0: e2_0,
		1: e1_1,
		2: e2_2,
	}, emitter.selectParents(oneParentAheadOneParentBehind, 3))
}

func TestPartialSynchronyEmitter_supportsThePrimaries_AlwaysSupportsGenesisRound(t *testing.T) {
	require := require.New(t)

	emitter := &PartialSynchronyEmitter{}

	require.True(emitter.supportsThePrimaries(nil, 1))
}

func TestPartialSynchronyEmitter_supportsThePrimaries_EvaluatesLeaderSupportCorrectly(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	const committeeSize = 4
	committee := consensus.NewUniformCommittee(committeeSize)
	layering := layering.NewMockLayering(ctrl)
	layering.EXPECT().GetRound(gomock.Any()).AnyTimes().DoAndReturn(func(event *model.Event) uint32 {
		return event.Seq()
	})
	layering.EXPECT().IsCandidate(gomock.Any()).AnyTimes().DoAndReturn(func(event *model.Event) bool {
		// Let Creator 2 be the primary for round 1 and Creator 3 for round 2
		return (event.Seq() == 1 && event.Creator() == 2) || (event.Seq() == 2 && event.Creator() == 3)
	})
	emitter := &PartialSynchronyEmitter{committee: committee, layering: layering}

	tests := map[string]struct {
		genesis  []consensus.ValidatorId
		parents  [][]int
		expected bool
	}{
		"supports later primary and does not observe the earlier": {
			// does not even OBSERVE 2 in round 1
			// but supports 3 in round 2
			genesis: []consensus.ValidatorId{0, 1, 3},
			parents: [][]int{
				{0, 1, 3},
				{1, 0, 3},
				nil,
				{3, 0, 1},
			},
			expected: false,
		},
		"supports no primary": {
			// does not quorum observe 2 in round 1
			// and does not observe 3 in round 2
			genesis: []consensus.ValidatorId{0, 1, 2, 3},
			parents: [][]int{
				{0, 1, 3},
				{1, 2, 3},
				{2, 0, 1},
				nil,
			},
			expected: false,
		},
		// does not quorum observe 2 in round 1
		// but observes 3 in round 2
		"supports later primary only": {
			genesis: []consensus.ValidatorId{0, 1, 2, 3},
			parents: [][]int{
				{0, 1, 3},
				{1, 2, 3},
				{2, 0, 1},
				{3, 0, 1},
			},
			expected: false,
		},
		// quorum observes 2 in round 1
		// but does not observe 3 in round 2
		"supports earlier primary only": {
			genesis: []consensus.ValidatorId{0, 1, 2, 3},
			parents: [][]int{
				{0, 2, 3},
				{1, 2, 3},
				{2, 0, 3},
				nil,
			},
			expected: false,
		},
		// quorum observes 2 in round 1
		// and observes 3 in round 2
		"supports both primaries": {
			genesis: []consensus.ValidatorId{0, 1, 2, 3},
			parents: [][]int{
				{0, 2, 3},
				{1, 2, 3},
				{2, 0, 3},
				{3, 0, 1},
			},
			expected: true,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			dag := model.NewDag(committee)

			// Create the genesis events
			genesis := make([]*model.Event, committeeSize)
			for _, validatorId := range testCase.genesis {
				events := dag.AddEvent(model.EventMessage{Creator: validatorId})
				require.Len(events, 1)
				genesis[validatorId] = events[0]
			}

			// Create the round 2 events according to the test case
			for i, parentIndices := range testCase.parents {
				if parentIndices == nil {
					continue
				}
				var parents []model.EventId
				for _, parentIndex := range parentIndices {
					parents = append(parents, genesis[parentIndex].EventId())
				}
				dag.AddEvent(model.EventMessage{Creator: consensus.ValidatorId(i), Parents: parents})
			}

			emitter.latestEmittedRound = 2
			emitter.dag = dag
			require.Equal(testCase.expected, emitter.supportsThePrimaries(dag.GetHeads(), 3))
		})
	}
}

func TestPartialSynchronyEmitter_startTimeoutTimer_SetsTimeoutFlagAfterDuration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		require := require.New(t)

		committee := consensus.NewUniformCommittee(1)
		dag := model.NewDag(committee)
		emitter := &PartialSynchronyEmitter{
			dag:             dag,
			creator:         0,
			timeoutDuration: 100 * time.Millisecond,
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

func TestPartialSynchronyEmitter_Stop_StopsAnyFurtherEmissions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		dag := model.NewMockDag(ctrl)
		// [model.Dag.GetHeads] should not be reached after Stop
		dag.EXPECT().GetHeads().Times(0)

		emitter := PartialSynchronyEmitterFactory{
			TimeoutDuration: 100 * time.Millisecond,
		}.NewEmitter(
			nil, dag, 0, nil,
		)

		emitter.Stop()
		time.Sleep(150 * time.Millisecond)
		emitter.OnChange()
	})
}

func TestPartialSynchronyEmitter_PartialSynchronyScenario(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		require := require.New(t)
		ctrl := gomock.NewController(t)

		emitChannel := NewMockChannel(ctrl)

		const primaryLeader, creator = consensus.ValidatorId(1), consensus.ValidatorId(0)
		const timeoutDuration = 500 * time.Millisecond

		committee := consensus.NewUniformCommittee(4)
		dag := model.NewDag(committee)
		layering := layering.NewMockLayering(ctrl)
		layering.EXPECT().GetRound(gomock.Any()).AnyTimes().DoAndReturn(func(event *model.Event) uint32 {
			return event.Seq()
		})
		layering.EXPECT().IsCandidate(gomock.Any()).AnyTimes().DoAndReturn(func(event *model.Event) bool {
			// Let Creator 2 be the primary for all rounds
			return event.Creator() == 2
		})

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

		readLastEmittedRoundAtomically := func(e *PartialSynchronyEmitter) uint32 {
			e.stateMutex.Lock()
			defer e.stateMutex.Unlock()
			return e.latestEmittedRound
		}

		emitter := newPartialSynchronyEmitter(dag, committee, creator, timeoutDuration, emitChannel, layering)

		emitter.OnChange() // genesis (1st emission)
		require.Equal(uint32(1), readLastEmittedRoundAtomically(emitter))

		// Attempt another emission right away, without updating the DAG, should not emit
		emitter.OnChange()
		require.Equal(uint32(1), readLastEmittedRoundAtomically(emitter))
		// e_#seq_#validator

		e1_0 := dag.GetHeads()[0]

		// Add another genesis event (primary), but should not emit due to lack of quorum
		e1_1 := dag.AddEvent(model.EventMessage{Creator: 1, Parents: []model.EventId{}})[0]
		emitter.OnChange()
		require.Equal(uint32(1), readLastEmittedRoundAtomically(emitter))

		time.Sleep(timeoutDuration / 2)
		// Quorum of round 1 parents observed, primary observed + timer NOT triggered (2nd emission)
		e1_2 := dag.AddEvent(model.EventMessage{Creator: 2, Parents: []model.EventId{}})[0]
		emitter.OnChange()
		require.Equal(uint32(2), readLastEmittedRoundAtomically(emitter))

		// Last genesis event, does not trigger any conditions.
		e1_3 := dag.AddEvent(model.EventMessage{Creator: 3, Parents: []model.EventId{}})[0]
		emitter.OnChange()
		require.Equal(uint32(2), readLastEmittedRoundAtomically(emitter))

		// Primary creator event with Seq 2 is added, no conditions met.
		dag.AddEvent(model.EventMessage{Creator: 1, Parents: []model.EventId{e1_1.EventId(), e1_0.EventId(), e1_2.EventId()}})
		emitter.OnChange()
		require.Equal(uint32(2), readLastEmittedRoundAtomically(emitter))

		// Quorum of round 2 parents reached, but no primary obsesrved by quorum of them yet.
		dag.AddEvent(model.EventMessage{Creator: 2, Parents: []model.EventId{e1_2.EventId(), e1_3.EventId(), e1_0.EventId()}})
		emitter.OnChange()
		require.Equal(uint32(2), readLastEmittedRoundAtomically(emitter))

		time.Sleep(timeoutDuration + 101*time.Millisecond)
		// Quorum of round 2 parents reached + timeout triggered and (primary observed by quorum) condition NOT met (3rd emission)
		require.Equal(uint32(3), readLastEmittedRoundAtomically(emitter))

		emitter.Stop()
	})
}
