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

func TestObservesLatesEmission_Evaluate(t *testing.T) {
	require := require.New(t)

	condition := NewObservesLatestEmissionCondition()
	emitter := getSimpleEmitter(t, condition)

	// Should always succeed for the genesis emission
	require.True(condition.Evaluate(emitter))

	emitter.lastEmittedSeq = 1
	// last emitted seq is 1, but dag is not updated
	require.False(condition.Evaluate(emitter))

	emitter.dag.AddEvent(model.EventMessage{})
	// emit as the latest emitted event is now in the dag
	require.True(condition.Evaluate(emitter))
}

func TestTimeoutCondition_Evaluate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		require := require.New(t)
		ctrl := gomock.NewController(t)

		const interval = 200 * time.Millisecond
		condition := NewTimeoutCondition(interval)

		emitter := getSimpleEmitter(t, condition)
		channel := NewMockChannel(ctrl)
		emitter.channel = channel
		channel.EXPECT().Emit(gomock.Any()).Times(5)

		condition.Reset(emitter)

		time.Sleep(interval / 2)
		require.False(condition.Evaluate(emitter))

		time.Sleep(interval * 5)

		emitter.Stop()
		time.Sleep(1 * time.Hour)
	})
}

func TestObservesNewParentsCondition_Evaluate(t *testing.T) {
	require := require.New(t)

	condition := NewObservesNewParentsCondition(2)
	emitter := getSimpleEmitter(t, condition)

	// Should always succeed for the genesis emission
	require.True(condition.Evaluate(emitter))

	emitter.dag.AddEvent(model.EventMessage{Creator: 0})
	// only 1 parent observed
	require.False(condition.Evaluate(emitter))

	emitter.dag.AddEvent(model.EventMessage{Creator: 1})
	// 2 parents observed
	require.True(condition.Evaluate(emitter))
}

func TestOrCondition_Evaluate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	trueCondition := NewMockCondition(ctrl)
	trueCondition.EXPECT().Evaluate(gomock.Any()).Return(true).AnyTimes()
	falseCondition := NewFalseCondition()
	emitter := getSimpleEmitter(t, nil)

	twoTrues := NewOrCondition(trueCondition, trueCondition)
	require.True(twoTrues.Evaluate(emitter))

	trueAndFalse := NewOrCondition(trueCondition, falseCondition)
	require.True(trueAndFalse.Evaluate(emitter))

	twoFalses := NewOrCondition(falseCondition, falseCondition)
	require.False(twoFalses.Evaluate(emitter))
}

func TestAndCondition_Evaluate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	trueCondition := NewMockCondition(ctrl)
	trueCondition.EXPECT().Evaluate(gomock.Any()).Return(true).AnyTimes()
	falseCondition := NewFalseCondition()
	emitter := getSimpleEmitter(t, nil)

	twoTrues := NewAndCondition(trueCondition, trueCondition)
	require.True(twoTrues.Evaluate(emitter))

	trueAndFalse := NewAndCondition(trueCondition, falseCondition)
	require.False(trueAndFalse.Evaluate(emitter))

	twoFalses := NewAndCondition(falseCondition, falseCondition)
	require.False(twoFalses.Evaluate(emitter))
}

func getSimpleEmitter(t *testing.T, condition Condition) *Emitter {
	t.Helper()

	dag := model.NewDag(consensus.NewUniformCommittee(3))
	return &Emitter{
		dag:                dag,
		lastEmittedParents: make(map[consensus.ValidatorId]*model.Event),
		condition:          condition,
	}
}
