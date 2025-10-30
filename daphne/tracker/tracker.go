package tracker

import (
	"time"

	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
)

//go:generate mockgen -source tracker.go -destination=tracker_mock.go -package=tracker

// Tracker facilitates the tracking of events throughout the application. The
// collected information can be used for debugging, monitoring, or analytics.
type Tracker interface {
	// Track records an event with optional metadata. The metadata can be any
	// key-value pairs that provide additional context about the event. If
	// keys show up multiple times, the last value will be used.
	Track(mark mark.Mark, meta ...any)

	// With creates a new Tracker instance that adds the provided metadata to
	// all events tracked by the resulting Tracker. This allows for components
	// to build a hierarchical tracking structure, where each component can
	// add its own context to the events.
	With(meta ...any) Tracker
}

// Sink represents a destination for tracked events. It defines methods to
// append new entries and to stop the sink when it's no longer needed.
type Sink interface {
	Append(entry *Entry)
	Close() error
}

// New creates a new root Tracker instance. This is the starting point for
// tracking events and also responsible for managing the collection and storage
// of tracked events. Observed events are forwarded to the provided Sink.
func New(sink Sink) *rootTracker {
	return &rootTracker{
		sink: sink,
	}
}

type rootTracker struct {
	sink Sink
}

func (r *rootTracker) Close() error {
	if r.sink == nil {
		return nil
	}
	err := r.sink.Close()
	r.sink = nil
	return err
}

func (r *rootTracker) Track(mark mark.Mark, meta ...any) {
	if r.sink == nil {
		return
	}
	r.sink.Append(&Entry{
		Time: time.Now(),
		Mark: mark,
		Meta: toMeta(meta...),
	})
}

func (r *rootTracker) With(meta ...any) Tracker {
	newMeta := toMeta(meta...)
	return &tracker{
		parent: r,
		meta:   newMeta,
	}
}

// -- sub-tracker implementation --

type tracker struct {
	parent Tracker
	meta   Metadata
}

func (t *tracker) Track(event mark.Mark, meta ...any) {
	m := toMeta(meta...)
	t.parent.Track(event, m.merge(t.meta).flatten()...)
}

func (t *tracker) With(meta ...any) Tracker {
	return &tracker{
		parent: t,
		meta:   toMeta(meta...),
	}
}
