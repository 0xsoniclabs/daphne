// Copyright 2026 Sonic Operations Ltd
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

package tracker_test

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestTracker_RecordEvents(t *testing.T) {
	require := require.New(t)
	collector := &collectingSink{}
	root := tracker.New(collector)

	sent := mark.Mark(0)
	received := mark.Mark(1)

	root.Track(sent, "msg", 123)
	root.Track(received, "msg", 456)

	entries := collector.entries
	require.Len(entries, 2)

	require.Equal(sent, entries[0].Mark)
	require.Equal("{msg: 123}", entries[0].Meta.String())

	require.Equal(received, entries[1].Mark)
	require.Equal("{msg: 456}", entries[1].Meta.String())
}

func TestTracker_CloseClosesSink(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sink := tracker.NewMockSink(ctrl)

	sink.EXPECT().Close().Return(nil)

	root := tracker.New(sink)
	require.NoError(root.Close())
}

func TestTracker_ClosedTracker_DoesNoLongerForwardTrackedEventsToSink(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sink := tracker.NewMockSink(ctrl)

	sink.EXPECT().Close().Return(nil)
	root := tracker.New(sink)
	require.NoError(root.Close())

	// No expectation set on sink.Append, so if this is called the test will fail.
	root.Track(mark.Mark(0), "msg", 123)

	// Also, closing again should be a no-op.
	require.NoError(root.Close())
}

func TestTracker_Track_LastKeyOverridesPreviousValues(t *testing.T) {
	require := require.New(t)
	collector := &collectingSink{}
	root := tracker.New(collector)

	event := mark.Mark(12)
	root.Track(event, "msg", 123, "msg", 456)

	entries := collector.entries
	require.Len(entries, 1)

	require.Equal(event, entries[0].Mark)
	require.Equal("{msg: 456}", entries[0].Meta.String())
}

func TestTracker_WithMeta_ProducesSubTrackerReportingExtraMeta(t *testing.T) {
	require := require.New(t)
	collector := &collectingSink{}
	root := tracker.New(collector)

	eventA := mark.Mark(0)
	eventB := mark.Mark(1)
	eventC := mark.Mark(2)
	eventD := mark.Mark(3)

	root.Track(eventA, "msg", 123)

	sub1Tracker := root.With("sub", 1)
	sub1Tracker.Track(eventB, "msg", 456)

	sub2Tracker := root.With("sub", 2)
	sub2Tracker.Track(eventC, "msg", 789)

	sub13Tracker := sub1Tracker.With("nested", 3)
	sub13Tracker.Track(eventD, "key", 1, "value", 2)

	entries := collector.entries
	require.Len(entries, 4)

	require.Equal(eventA, entries[0].Mark)
	require.Equal("{msg: 123}", entries[0].Meta.String())

	require.Equal(eventB, entries[1].Mark)
	require.Equal("{msg: 456, sub: 1}", entries[1].Meta.String())

	require.Equal(eventC, entries[2].Mark)
	require.Equal("{msg: 789, sub: 2}", entries[2].Meta.String())

	require.Equal(eventD, entries[3].Mark)
	require.Equal("{key: 1, nested: 3, sub: 1, value: 2}", entries[3].Meta.String())
}

func TestTracker_MetaOfSubTrackerCanNotBeOverridden(t *testing.T) {
	require := require.New(t)
	collector := &collectingSink{}
	root := tracker.New(collector)

	event := mark.Mark(12)

	sub := root.With("msg", 123)
	sub.Track(event, "msg", 456) // this should be ignored

	entries := collector.entries
	require.Len(entries, 1)

	require.Equal(event, entries[0].Mark)
	require.Equal("{msg: 123}", entries[0].Meta.String())
}

func TestTracker_Mock_CanBeChained(t *testing.T) {
	ctrl := gomock.NewController(t)
	t1 := tracker.NewMockTracker(ctrl)
	t2 := tracker.NewMockTracker(ctrl)

	event := mark.Mark(12)
	t1.EXPECT().With("key", "value").Return(t2)
	t2.EXPECT().Track(event, "key", "value")

	t1.With("key", "value").Track(event, "key", "value")
}

type collectingSink struct {
	entries []*tracker.Entry
}

func (c *collectingSink) Append(entry *tracker.Entry) {
	c.entries = append(c.entries, entry)
}

func (c *collectingSink) Close() error {
	return nil
}
