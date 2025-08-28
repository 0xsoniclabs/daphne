package tracker_test

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTracker_RecordEvents(t *testing.T) {
	require := require.New(t)
	root := tracker.New()

	root.Track("MsgA", "msg", 123)
	root.Track("MsgB", "msg", 456)

	entries := root.GetAll()
	require.Len(entries, 2)

	require.Equal("MsgA", entries[0].Event)
	require.Equal("{msg: 123}", entries[0].Meta.String())

	require.Equal("MsgB", entries[1].Event)
	require.Equal("{msg: 456}", entries[1].Meta.String())
}

func TestTracker_Track_LastKeyOverridesPreviousValues(t *testing.T) {
	require := require.New(t)
	root := tracker.New()

	root.Track("Msg", "msg", 123, "msg", 456)

	entries := root.GetAll()
	require.Len(entries, 1)

	require.Equal("Msg", entries[0].Event)
	require.Equal("{msg: 456}", entries[0].Meta.String())
}

func TestTracker_WithMeta_ProducesSubTrackerReportingExtraMeta(t *testing.T) {
	require := require.New(t)
	root := tracker.New()

	root.Track("MsgA", "msg", 123)

	sub1Tracker := root.With("sub", 1)
	sub1Tracker.Track("MsgB", "msg", 456)

	sub2Tracker := root.With("sub", 2)
	sub2Tracker.Track("MsgC", "msg", 789)

	sub13Tracker := sub1Tracker.With("nested", 3)
	sub13Tracker.Track("MsgD", "key", 1, "value", 2)

	entries := root.GetAll()
	require.Len(entries, 4)

	require.Equal("MsgA", entries[0].Event)
	require.Equal("{msg: 123}", entries[0].Meta.String())

	require.Equal("MsgB", entries[1].Event)
	require.Equal("{msg: 456, sub: 1}", entries[1].Meta.String())

	require.Equal("MsgC", entries[2].Event)
	require.Equal("{msg: 789, sub: 2}", entries[2].Meta.String())

	require.Equal("MsgD", entries[3].Event)
	require.Equal("{key: 1, nested: 3, sub: 1, value: 2}", entries[3].Meta.String())
}

func TestTracker_MetaOfSubTrackerCanNotBeOverridden(t *testing.T) {
	require := require.New(t)
	root := tracker.New()

	sub := root.With("msg", 123)
	sub.Track("Attention", "msg", 456) // this should be ignored

	entries := root.GetAll()
	require.Len(entries, 1)

	require.Equal("Attention", entries[0].Event)
	require.Equal("{msg: 123}", entries[0].Meta.String())
}

func TestTracker_Mock_CanBeChained(t *testing.T) {
	ctrl := gomock.NewController(t)
	t1 := tracker.NewMockTracker(ctrl)
	t2 := tracker.NewMockTracker(ctrl)

	t1.EXPECT().With("key", "value").Return(t2)
	t2.EXPECT().Track("Event", "key", "value")

	t1.With("key", "value").Track("Event", "key", "value")
}
