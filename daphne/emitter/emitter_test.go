package emitter

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEmitter_Stop_StopsEmitterLoopAndReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	channel := broadcast.NewMockChannel[string](ctrl)
	payloadSource := NewMockEmissionPayloadSource[string](ctrl)

	emitter := StartSimpleEmitter(payloadSource, channel, 0)
	emitter.Stop()
}

func TestEmitter_StartSimpleEmitter_EmitsAtInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		source := NewMockEmissionPayloadSource[int](ctrl)
		channel := broadcast.NewMockChannel[int](ctrl)

		const (
			emitInterval = 10 * time.Millisecond
			numEmissions = 5
		)

		source.EXPECT().GetEmissionPayload().Return(1).AnyTimes()

		done := make(chan struct{})
		numSeenEvents := 0
		lastTime := time.Now()
		channel.EXPECT().Broadcast(gomock.Any()).Do(func(int) {
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
		}).Times(numEmissions)

		emitter := StartSimpleEmitter(source, channel, emitInterval)
		defer emitter.Stop()

		select {
		case <-done:
			// All expected broadcasts were seen.
		case <-time.After(emitInterval * (numEmissions + 1)):
			t.Error("Timed out waiting for broadcasts.")
		}
	})
}
