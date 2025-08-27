package generic

import (
	"time"
)

const (
	// DefaultEmitInterval is the default interval for emitting new bundles
	// if one is not specified in the configuration.
	DefaultEmitInterval = 500 * time.Millisecond
)

// EmissionPayloadSource is a payload provider for the emitter.
type EmissionPayloadSource[T any] interface {
	// GetEmissionPayload returns the next payload to be emitted.
	// It should never be blocking.
	GetEmissionPayload() T
}

// Emitter is a component that periodically emits messages from
// a specified source, at a specified interval.
type Emitter[T any] struct {
	quit chan<- struct{}
	done <-chan struct{}
}

// StartEmitter creates and starts an instance of Emitter with the provided emitInterval,
// getEmissionPayload function which provides concrete messages on-demand, and a gossip
// instance through which the messages will be broadcasted. It returns the started
// Emitter instance through which the emission loop can be stopped.
func StartEmitter[T any](
	emitInterval time.Duration,
	source EmissionPayloadSource[T],
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
				payload := source.GetEmissionPayload()
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
