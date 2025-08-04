package tracker_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestTracker_ExportCSV_ProducesOutputWithProperColumnLabeling(t *testing.T) {
	require := require.New(t)
	root := tracker.New()

	root.Track("MsgA", "key1", 123)
	root.Track("MsgB", "key1", 456, "key2", "value2")
	root.Track("MsgC", "key2", 3.1415)

	var output bytes.Buffer
	err := root.ExportCSV(&output)
	require.NoError(err)

	csv := output.String()
	require.NotEmpty(csv)
	require.Contains(csv, "Time,Event,key1,key2\n")
	require.Contains(csv, ",MsgA,123,\n")
	require.Contains(csv, ",MsgB,456,value2\n")
	require.Contains(csv, ",MsgC,,3.1415\n")
}

func TestTracker_ExportCSV_UsesUnixNanosecondsForTimestamps(t *testing.T) {
	require := require.New(t)
	root := tracker.New()

	root.Track("MsgA")
	root.Track("MsgB")
	root.Track("MsgC")

	var output bytes.Buffer
	err := root.ExportCSV(&output)
	require.NoError(err)

	events := root.GetAll()

	csv := output.String()
	require.NotEmpty(csv)
	require.Contains(csv, "Time,Event\n")
	require.Contains(csv, fmt.Sprintf("%d,MsgA\n", events[0].Time.UnixNano()))
	require.Contains(csv, fmt.Sprintf("%d,MsgB\n", events[1].Time.UnixNano()))
	require.Contains(csv, fmt.Sprintf("%d,MsgC\n", events[2].Time.UnixNano()))
}

func TestTracker_ExportCSV_WriterIssuesAreReported(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	root := tracker.New()

	// Use large enough keys and values to exceed the buffer size of the CSV writer.
	largeKey := string(make([]byte, 10_000))
	largeValue := string(make([]byte, 10_000))

	root.Track("MsgA", largeKey, largeValue)
	root.Track("MsgB", largeKey, largeValue)
	root.Track("MsgC", largeKey, largeValue)

	// count the number of writes
	counter := 0
	mockWriter := tracker.NewMockWriter(ctrl)
	mockWriter.EXPECT().Write(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
		counter++
		return len(b), nil
	}).AnyTimes()

	require.NoError(root.ExportCSV(mockWriter))
	require.Greater(counter, 0)

	for i := range counter - 1 {
		t.Run(fmt.Sprintf("failAfterWrite=%d", i), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockWriter := tracker.NewMockWriter(ctrl)

			issue := fmt.Errorf("injected error")
			gomock.InOrder(
				mockWriter.EXPECT().Write(gomock.Any()).DoAndReturn(
					func(b []byte) (int, error) {
						return len(b), nil
					},
				).Times(i),
				mockWriter.EXPECT().Write(gomock.Any()).Return(0, issue),
			)

			require.ErrorIs(root.ExportCSV(mockWriter), issue)
		})
	}
}
