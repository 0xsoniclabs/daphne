package tracker_test

/*
import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTracker_RecordEvents(t *testing.T) {
	require := require.New(t)
	root := tracker.New()

	sent := mark.Mark(0)
	received := mark.Mark(1)

	root.Track(sent, "msg", 123)
	root.Track(received, "msg", 456)

	entries := root.GetAll()
	require.Len(entries, 2)

	require.Equal(sent, entries[0].Mark)
	require.Equal("{msg: 123}", entries[0].Meta.String())

	require.Equal(received, entries[1].Mark)
	require.Equal("{msg: 456}", entries[1].Meta.String())
}

func TestTracker_Track_LastKeyOverridesPreviousValues(t *testing.T) {
	require := require.New(t)
	root := tracker.New()

	event := mark.Mark(12)
	root.Track(event, "msg", 123, "msg", 456)

	entries := root.GetAll()
	require.Len(entries, 1)

	require.Equal(event, entries[0].Mark)
	require.Equal("{msg: 456}", entries[0].Meta.String())
}

func TestTracker_WithMeta_ProducesSubTrackerReportingExtraMeta(t *testing.T) {
	require := require.New(t)
	root := tracker.New()

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

	entries := root.GetAll()
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
	root := tracker.New()

	event := mark.Mark(12)

	sub := root.With("msg", 123)
	sub.Track(event, "msg", 456) // this should be ignored

	entries := root.GetAll()
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
*/
