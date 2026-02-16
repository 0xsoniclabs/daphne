// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package tracker

import (
	"sync/atomic"
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
	res := &rootTracker{}
	res.sink.Store(&sink)
	return res
}

type rootTracker struct {
	sink atomic.Pointer[Sink]
}

func (r *rootTracker) Close() error {
	sinkPointer := r.sink.Swap(nil)
	if sinkPointer == nil {
		return nil
	}
	return (*sinkPointer).Close()
}

func (r *rootTracker) Track(mark mark.Mark, meta ...any) {
	if ptr := r.sink.Load(); ptr != nil {
		(*ptr).Append(&Entry{
			Time: time.Now(),
			Mark: mark,
			Meta: toMeta(meta...),
		})
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
