package tracker

import (
	"slices"
	"sync"
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

// New creates a new root Tracker instance. This is the starting point for
// tracking events and also responsible for managing the collection and storage
// of tracked events.
func New() *rootTracker {
	return &rootTracker{}
}

type rootTracker struct {
	entries []Entry
	mutex   sync.Mutex
}

// GetAll returns all tracked entries. The result is a snapshot of the current
// events and is safe to use concurrently.
func (r *rootTracker) GetAll() []Entry {
	// Perform a fast, shallow copy of the entries. Since metadata is immutable
	// outside of this package, we can safely return a slice of the entries.
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return slices.Clone(r.entries)
}

func (r *rootTracker) Track(mark mark.Mark, meta ...any) {
	entry := Entry{
		Time: time.Now(),
		Mark: mark,
		Meta: toMeta(meta...),
	}
	r.mutex.Lock()
	r.entries = append(r.entries, entry)
	r.mutex.Unlock()
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
