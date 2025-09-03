package generic

import (
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
)

//go:generate mockgen -source emitter.go -destination=emitter_mock.go -package=generic

const (
	// DefaultEmitInterval is the default interval for emitting new bundles
	// if one is not specified in the configuration.
	DefaultEmitInterval = 500 * time.Millisecond
)

// EmissionPayloadSource is a payload provider for the emitter.
type EmissionPayloadSource[T any] interface {
	// GetEmissionPayload returns the next payload to be emitted.
	// It should return as soon as possible without blocking or busy waiting on
	// internal resources.
	GetEmissionPayload() T
}

// Emitter is a component that periodically emits messages from
// a specified source, at a specified interval.
type Emitter[T any] struct {
	job concurrent.Job
}

// StartSimpleEmitter creates and starts an instance of Emitter with the provided
// getEmissionPayload function which provides concrete messages on-demand, a gossip
// instance through which the messages will be broadcasted and an emit interval.
// It returns the started Emitter instance through which the emission loop can be stopped.
func StartSimpleEmitter[T any](
	source EmissionPayloadSource[T],
	gossip Broadcaster[T],
	emitInterval time.Duration,
) *Emitter[T] {
	task := func(_ time.Time, source EmissionPayloadSource[T], gossip Broadcaster[T]) {
		payload := source.GetEmissionPayload()
		gossip.Broadcast(payload)
	}
	return StartCustomEmitter(emitInterval, source, gossip, task)
}

// StartCustomEmitter creates and starts an instance of Emitter with the provided
// custom task function which is executed at the specified emit interval, utilizing
// the provided source and gossip instances. It returns the started Emitter instance
// through which the emission loop can be stopped.
func StartCustomEmitter[T any](
	emitInterval time.Duration,
	source EmissionPayloadSource[T],
	gossip Broadcaster[T],
	task func(time.Time, EmissionPayloadSource[T], Broadcaster[T]),
) *Emitter[T] {
	if emitInterval == 0 {
		emitInterval = DefaultEmitInterval
	}
	wrapper := func(t time.Time) {
		task(t, source, gossip)
	}
	return &Emitter[T]{job: *concurrent.StartPeriodicJob(emitInterval, wrapper)}
}

// Stop signals the emitter instance to stop and blocks until its emission loop
// exits.
func (e *Emitter[T]) Stop() {
	e.job.Stop()
}
