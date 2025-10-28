package tracker

import (
	"io"
	"log/slog"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
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
func New(out io.Writer) *rootTracker {
	entries := make(chan *Entry, 100)
	job := concurrent.StartJob(func(stop <-chan struct{}) {
	loop:
		for {
			select {
			case <-stop:
				break loop
			case entry := <-entries:
				if entry == nil {
					break loop
				}
				if err := ExportAsJson(*entry, out); err != nil {
					// TODO: track errors properly
					slog.Warn("Failed to export tracker entry", "error", err)
				}
				_, err := out.Write([]byte("\n"))
				if err != nil {
					slog.Warn("Failed to write line terminator", "error", err)
					break loop
				}
			}
		}
		// in case of an error, consume the remaining entries
		for range entries {
		}
	})
	return &rootTracker{
		entryChannel: entries,
		writerJob:    job,
	}
}

type rootTracker struct {
	entryChannel chan<- *Entry
	writerJob    *concurrent.Job
}

func (r *rootTracker) Stop() {
	if r.entryChannel == nil {
		return
	}
	close(r.entryChannel)
	r.entryChannel = nil
	r.writerJob.Stop()
	r.writerJob = nil
}

func (r *rootTracker) Track(mark mark.Mark, meta ...any) {
	r.entryChannel <- &Entry{
		Time: time.Now(),
		Mark: mark,
		Meta: toMeta(meta...),
	}
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
