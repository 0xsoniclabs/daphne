package tracker

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTracker_ExportCSV_ProducesOutputWithProperColumnLabeling(t *testing.T) {
	require := require.New(t)
	root := New()

	root.Track(mark.Mark(10_000), "key1", 123)
	root.Track(mark.Mark(10_001), "key1", 456, "key2", "value2")
	root.Track(mark.Mark(10_002), "key2", 3.1415)

	var output bytes.Buffer
	err := ExportAsCSV(root.GetAll(), &output)
	require.NoError(err)

	csv := output.String()
	require.NotEmpty(csv)
	require.Contains(csv, "timestamp,mark,key1,key2\n")
	require.Contains(csv, ",Mark(10000),123,\n")
	require.Contains(csv, ",Mark(10001),456,value2\n")
	require.Contains(csv, ",Mark(10002),,3.1415\n")
}

func TestTracker_ExportCSV_UsesUnixNanosecondsForTimestamps(t *testing.T) {
	require := require.New(t)
	root := New()

	root.Track(mark.Mark(10_000))
	root.Track(mark.Mark(10_001))
	root.Track(mark.Mark(10_002))

	var output bytes.Buffer
	err := ExportAsCSV(root.GetAll(), &output)
	require.NoError(err)

	events := root.GetAll()

	csv := output.String()
	require.NotEmpty(csv)
	require.Contains(csv, "timestamp,mark\n")
	require.Contains(csv, fmt.Sprintf("%d,Mark(10000)\n", events[0].Time.UnixNano()))
	require.Contains(csv, fmt.Sprintf("%d,Mark(10001)\n", events[1].Time.UnixNano()))
	require.Contains(csv, fmt.Sprintf("%d,Mark(10002)\n", events[2].Time.UnixNano()))
}

func TestTracker_ExportCSV_WriterIssuesAreReported(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	root := New()

	// Use large enough keys and values to exceed the buffer size of the CSV writer.
	largeKey := string(make([]byte, 10_000))
	largeValue := string(make([]byte, 10_000))

	root.Track(mark.Mark(10_000), largeKey, largeValue)
	root.Track(mark.Mark(10_001), largeKey, largeValue)
	root.Track(mark.Mark(10_002), largeKey, largeValue)

	// count the number of writes
	counter := 0
	mockWriter := NewMock_Writer(ctrl)
	mockWriter.EXPECT().Write(gomock.Any()).DoAndReturn(func(b []byte) (int, error) {
		counter++
		return len(b), nil
	}).AnyTimes()

	require.NoError(ExportAsCSV(root.GetAll(), mockWriter))
	require.Greater(counter, 0)

	for i := range counter - 1 {
		t.Run(fmt.Sprintf("failAfterWrite=%d", i), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockWriter := NewMock_Writer(ctrl)

			issue := fmt.Errorf("injected error")
			gomock.InOrder(
				mockWriter.EXPECT().Write(gomock.Any()).DoAndReturn(
					func(b []byte) (int, error) {
						return len(b), nil
					},
				).Times(i),
				mockWriter.EXPECT().Write(gomock.Any()).Return(0, issue),
			)

			require.ErrorIs(ExportAsCSV(root.GetAll(), mockWriter), issue)
		})
	}
}
