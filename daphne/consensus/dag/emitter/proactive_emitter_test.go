package emitter

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestProactiveEmitterFactory_IsAnEmitterFactoryImplementation(t *testing.T) {
	var _ Factory = ProactiveEmitterFactory{}
}

func TestProactiveEmitterFactory_String_ProducesReadableSummary(t *testing.T) {
	factory := ProactiveEmitterFactory{NumNewParents: 3}
	require.Equal(t, "proactive_3", factory.String())
}

func TestProactiveEmitter_IsAnEmitterImplementation(t *testing.T) {
	var _ Emitter = &ProactiveEmitter{}
}

func TestProactiveEmitter_OnChange_AlwaysEmitsGenesisEvent(t *testing.T) {
	ctrl := gomock.NewController(t)

	dag := model.NewMockDag(ctrl)
	channel := NewMockChannel(ctrl)
	emitter := newProactiveEmitter(channel, dag, 0, 1, nil)

	dag.EXPECT().GetHeads().Return(map[consensus.ValidatorId]*model.Event{})
	channel.EXPECT().Emit(gomock.Any())

	emitter.OnChange()
	require.Equal(t, uint32(1), emitter.lastEmittedSeq)
	require.Empty(t, emitter.lastEmittedParents)
}

func TestProactiveEmitter_OnChange_RequiresLastEmissionToEmit(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	dag := model.NewMockDag(ctrl)
	channel := NewMockChannel(ctrl)
	emitter := newProactiveEmitter(channel, dag, 0, 1, nil)
	// Provide the new parents for the emission, but the emission with seq 1
	// is not present in the dag yet.

	emitter.lastEmittedSeq = 1
	event, err := model.NewEvent(1, nil, nil, time.Time{})
	require.NoError(err)
	dag.EXPECT().GetHeads().Return(map[consensus.ValidatorId]*model.Event{1: event})

	emitter.OnChange()
	require.Equal(uint32(1), emitter.lastEmittedSeq)
	require.Empty(emitter.lastEmittedParents)

	// The creators latest event is now in the dag.
	creatorEvent, err := model.NewEvent(0, nil, nil, time.Time{})
	require.NoError(err)
	dag.EXPECT().GetHeads().Return(map[consensus.ValidatorId]*model.Event{0: creatorEvent, 1: event})

	channel.EXPECT().Emit(gomock.Any())
	emitter.OnChange()
	require.Equal(uint32(2), emitter.lastEmittedSeq)
	require.Equal(map[consensus.ValidatorId]*model.Event{0: creatorEvent, 1: event}, emitter.lastEmittedParents)

	eventNew, err := model.NewEvent(1, []*model.Event{event}, nil, time.Time{})
	require.NoError(err)
	dag.EXPECT().GetHeads().Return(map[consensus.ValidatorId]*model.Event{0: creatorEvent, 1: eventNew})

	// Expect no emission since the creator's latest event has not advanced.
	emitter.OnChange()
}

func TestProactiveEmitte_OnChange_RequiresNewParentsToEmit(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	dag := model.NewMockDag(ctrl)
	channel := NewMockChannel(ctrl)
	emitter := newProactiveEmitter(channel, dag, 0, 2, nil)

	emitter.lastEmittedSeq = 1
	// Provide the creator's latest event for the next emission, but not enough
	// new events are in dag to fulfill the emission condition.
	creatorEvent, err := model.NewEvent(0, nil, nil, time.Time{})
	require.NoError(err)
	dag.EXPECT().GetHeads().Return(map[consensus.ValidatorId]*model.Event{0: creatorEvent})

	emitter.OnChange()
	require.Equal(uint32(1), emitter.lastEmittedSeq)
	require.Empty(emitter.lastEmittedParents)

	// There are 2 new parents now, so the emission is expected.
	event, err := model.NewEvent(1, nil, nil, time.Time{})
	require.NoError(err)
	dag.EXPECT().GetHeads().Return(map[consensus.ValidatorId]*model.Event{0: creatorEvent, 1: event})

	channel.EXPECT().Emit(gomock.Any())
	emitter.OnChange()
	require.Equal(uint32(2), emitter.lastEmittedSeq)
	require.Equal(map[consensus.ValidatorId]*model.Event{0: creatorEvent, 1: event}, emitter.lastEmittedParents)
}

func TestProactiveEmitter_Stop_RejectsFutureEmissions(t *testing.T) {
	ctrl := gomock.NewController(t)

	dag := model.NewMockDag(ctrl)
	channel := NewMockChannel(ctrl)
	emitter := ProactiveEmitterFactory{NumNewParents: 1}.NewEmitter(channel, dag, 0, nil)

	dag.EXPECT().GetHeads().Return(map[consensus.ValidatorId]*model.Event{})
	channel.EXPECT().Emit(gomock.Any())
	emitter.OnChange()

	emitter.Stop()

	// The emission attempt should not reach the [model.Dag.GetHeads] method.
	emitter.OnChange()
}
