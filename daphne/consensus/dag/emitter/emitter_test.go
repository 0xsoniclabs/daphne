package emitter

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEmitter_StartNewEmitter_InitializesFieldsAndResetsCondition(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)

	emitChannel := NewMockChannel(ctrl)
	condition := NewMockCondition(ctrl)

	dag := model.NewDag(consensus.NewUniformCommittee(1))
	const creator = consensus.ValidatorId(0)

	condition.EXPECT().Reset(gomock.Any())
	emitter := StartNewEmitter(creator, dag, emitChannel, condition)

	require.Equal(creator, emitter.creator)
	require.Equal(dag, emitter.dag)
	require.Equal(emitChannel, emitter.channel)
	require.Equal(uint32(0), emitter.lastEmittedSeq)
	require.Empty(emitter.lastEmittedParents)
}

func TestEmitter_AttemptEmission_DoesNotEmitOnNotMetCondition(t *testing.T) {
	ctrl := gomock.NewController(t)

	emitChannel := NewMockChannel(ctrl)
	condition := NewMockCondition(ctrl)
	dag := model.NewDag(consensus.NewUniformCommittee(1))

	condition.EXPECT().Reset(gomock.Any())
	emitter := StartNewEmitter(0, dag, emitChannel, condition)

	condition.EXPECT().Evaluate(gomock.Any()).Return(false)
	emitChannel.EXPECT().Emit(gomock.Any()).Times(0)
	emitter.AttemptEmission()
}

func TestEmitter_AttemptEmission_EmitsAndOnMetCondition(t *testing.T) {
	ctrl := gomock.NewController(t)

	emitChannel := NewMockChannel(ctrl)
	condition := NewMockCondition(ctrl)
	dag := model.NewDag(consensus.NewUniformCommittee(1))

	// One reset for initialization and the other one after emission
	condition.EXPECT().Reset(gomock.Any()).Times(2)

	emitter := StartNewEmitter(0, dag, emitChannel, condition)

	events := dag.AddEvent(model.EventMessage{Creator: 0})
	require.Len(t, events, 1)
	e := events[0]
	emitter.lastEmittedParents[e.Creator()] = e
	emitter.lastEmittedSeq = 1

	condition.EXPECT().Evaluate(gomock.Any()).Return(true)
	emitChannel.EXPECT().Emit(gomock.Any()).Times(1)
	emitter.AttemptEmission()

	require.Equal(t, uint32(2), emitter.lastEmittedSeq)
	require.Equal(t, e, emitter.lastEmittedParents[e.Creator()])
}

func TestEmitter_Stop_SetsConditionToFalse(t *testing.T) {
	ctrl := gomock.NewController(t)

	emitChannel := NewMockChannel(ctrl)
	condition := NewMockCondition(ctrl)
	dag := model.NewDag(consensus.NewUniformCommittee(1))

	condition.EXPECT().Reset(gomock.Any())
	emitter := StartNewEmitter(0, dag, emitChannel, condition)

	emitter.Stop()

	require.False(t, emitter.condition.Evaluate(emitter))
}
