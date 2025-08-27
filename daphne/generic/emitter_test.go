package generic

import (
	"testing"
	"time"

	"go.uber.org/mock/gomock"
)

func TestEmitter_Stop_StopsEmitterLoopAndReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	gossip := NewMockGossip[string](ctrl)
	payloadSource := NewMockEmissionPayloadSource[string](ctrl)

	emitter := StartEmitter(0, payloadSource, gossip)
	emitter.Stop()
}

func TestEmitter_StartEmitter_EmitsAtInterval(t *testing.T) {
	ctrl := gomock.NewController(t)

	gossip := NewMockGossip[int](ctrl)

	const (
		emitInterval = 200 * time.Millisecond
		numEmissions = 5
		waitInterval = emitInterval*numEmissions + emitInterval/2
	)

	for i := 1; i <= numEmissions; i++ {
		gossip.EXPECT().Broadcast(i)
	}

	emitter := StartEmitter(emitInterval, &incrementingPayloadSource{}, gossip)
	defer emitter.Stop()

	time.Sleep(waitInterval)
}

type incrementingPayloadSource struct {
	counter int
}

func (s *incrementingPayloadSource) GetEmissionPayload() int {
	s.counter++
	return s.counter
}
