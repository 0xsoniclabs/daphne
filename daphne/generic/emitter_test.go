package generic

import (
	"fmt"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
)

func TestEmitter_Stop_StopsEmitterLoopAndReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	gossip := NewMockGossip[string](ctrl)

	emitter := StartEmitter(0, func() string { return "" }, gossip)
	emitter.Stop()
}

func TestEmitter_StartEmitter_EmitsAtInterval(t *testing.T) {
	ctrl := gomock.NewController(t)

	gossip := NewMockGossip[string](ctrl)

	const (
		emitInterval  = 200 * time.Millisecond
		numEmissions  = 5
		waitInterval  = emitInterval*numEmissions + emitInterval/2
		payloadPrefix = "test "
	)

	for i := 1; i <= numEmissions; i++ {
		gossip.EXPECT().Broadcast(payloadPrefix + fmt.Sprint(i))
	}

	emissionCounter := 0
	emitter := StartEmitter(emitInterval, func() string {
		emissionCounter++
		return payloadPrefix + fmt.Sprint(emissionCounter)
	}, gossip)
	defer emitter.Stop()

	time.Sleep(waitInterval)
}
