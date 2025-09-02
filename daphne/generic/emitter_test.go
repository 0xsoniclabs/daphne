package generic

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEmitter_Stop_StopsEmitterLoopAndReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	gossip := NewMockBroadcaster[string](ctrl)
	payloadSource := NewMockEmissionPayloadSource[string](ctrl)

	emitter := StartEmitter(payloadSource, gossip, 0)
	emitter.Stop()
}

func TestEmitter_StartEmitter_EmitsAtInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		source := NewMockEmissionPayloadSource[int](ctrl)
		gossip := NewMockBroadcaster[int](ctrl)

		const (
			emitInterval = 10 * time.Millisecond
			numEmissions = 5
		)

		source.EXPECT().GetEmissionPayload().Return(1).AnyTimes()

		done := make(chan struct{})
		numSeenEvents := 0
		lastTime := time.Now()
		gossip.EXPECT().Broadcast(gomock.Any()).Do(func(int) {
			numSeenEvents++
			if numSeenEvents == numEmissions {
				close(done)
			}
			// Check that emissions are spaced out by the emit interval.
			now := time.Now()
			delta := time.Since(lastTime)
			if numSeenEvents == 1 {
				require.LessOrEqual(t, delta, emitInterval)
			} else {
				require.Equal(t, delta, emitInterval)
			}
			lastTime = now
		}).AnyTimes()

		emitter := StartEmitter(source, gossip, emitInterval)
		defer emitter.Stop()

		select {
		case <-done:
			// All expected broadcasts were seen.
		case <-time.After(emitInterval * (numEmissions + 1)):
			t.Error("Timed out waiting for broadcasts.")
		}
	})
}
