package generic

import (
	"time"
)

const (
	// DefaultEmitInterval is the default interval for emitting new bundles
	// if one is not specified in the configuration.
	DefaultEmitInterval = 500 * time.Millisecond
)

// GetEmissionPayloadFunc is a source of payloads for the emitter.
// It should not be blocking.
type GetEmissionPayloadFunc[T any] func() T

// Emitter is a component that periodically emits custom messages at a specified interval.
type Emitter[T any] struct {
	quit chan<- struct{}
	done <-chan struct{}
}

// StartEmitter creates and starts an instance of Emitter with the provided emitInterval,
// getEmissionPayload function, and gossip instance through which the emitted
// messages will be broadcasted.
func StartEmitter[T any](
	emitInterval time.Duration,
	getEmissionPayload GetEmissionPayloadFunc[T],
	gossip Gossip[T],
) *Emitter[T] {
	quit := make(chan struct{})
	done := make(chan struct{})

	if emitInterval == 0 {
		emitInterval = DefaultEmitInterval
	}

	go func() {
		defer close(done)
		for {
			select {
			case <-time.After(emitInterval):
				payload := getEmissionPayload()
				gossip.Broadcast(payload)
			// Keep emitting until we are signaled to stop.
			case <-quit:
				return
			}
		}
	}()

	return &Emitter[T]{
		quit: quit,
		done: done,
	}
}

// Stop signals the emitter instance to stop and blocks until its emission loop
// exits.
func (e *Emitter[T]) Stop() {
	if e.quit != nil {
		close(e.quit)
		e.quit = nil
		<-e.done
		e.done = nil
	}
}
