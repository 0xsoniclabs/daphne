package emitter

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPeriodicEmitterFactory_IsAnEmitterFactoryImplementation(t *testing.T) {
	var _ Factory = PeriodicEmitterFactory{}
}

func TestPeriodicEmitterFactory_String_ProducesReadableSummary(t *testing.T) {
	factory := PeriodicEmitterFactory{Interval: 150 * time.Millisecond}
	require.Equal(t, "periodic_150ms", factory.String())
}

func TestPeriodicEmitter_IsAnEmitterImplementation(t *testing.T) {
	var _ Emitter = &PeriodicEmitter{}
}

func TestPeriodicEmitter_NewEmitter_TriggersEmissionsPeriodicallyAndStops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		testInterval := 10 * time.Millisecond
		const numEmissions = 5

		dag := model.NewMockDag(ctrl)
		dag.EXPECT().GetHeads().Times(numEmissions)

		channel := NewMockChannel(ctrl)
		channel.EXPECT().Emit(gomock.Any()).Times(numEmissions)

		emitter := PeriodicEmitterFactory{Interval: testInterval}.NewEmitter(channel, dag, 0, nil)
		time.Sleep(testInterval * numEmissions)

		emitter.Stop()
	})
}
