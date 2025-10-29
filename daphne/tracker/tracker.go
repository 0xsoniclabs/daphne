package tracker

import (
	"fmt"
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
func New(path string) (*rootTracker, error) {
	entries := make(chan *Entry, 100)
	out, err := newParquetExporter(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create parquet exporter: %w", err)
	}
	job := concurrent.StartJob(func(stop <-chan struct{}) {
		for running := true; running; {
			select {
			case <-stop:
				running = false
			case entry := <-entries:
				if entry == nil {
					running = false
					continue
				}
				if err := out.append(entry); err != nil {
					// TODO: track errors properly
					slog.Warn("Failed to export tracker entry", "error", err)
				}
			}
		}
		// in case of an error, consume the remaining entries in the channel
		for range entries {
		}
		if err := out.close(); err != nil {
			slog.Warn("Failed to close tracker exporter", "error", err)
		}

	})
	return &rootTracker{
		entryChannel: entries,
		writerJob:    job,
	}, nil
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
