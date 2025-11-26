package tracker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

// --- ParquetSink tests ---

func TestParquetSink_StartsAndStopsWithoutError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.parquet")
	sink, err := NewParquetSink(path)
	require.NoError(t, err)
	require.NoError(t, sink.Close())
}

func TestNewParquetSink_InvalidFilePath_ProducesAnErrorOnCreation(t *testing.T) {
	invalidPath := filepath.Join(t.TempDir(), "nonexistent_dir", "test.parquet")
	_, err := NewParquetSink(invalidPath)
	require.ErrorContains(t, err, "no such file or directory")
}

func TestParquetSink_ErrorDuringAppend_ErrorsAreCollectedAndReportedOnClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	target := NewMock_sinkTarget(ctrl)
	sink := startParquetSink(target)

	issueA := fmt.Errorf("injected issue A")
	issueB := fmt.Errorf("injected issue B")
	target.EXPECT().append(gomock.Any()).Return(issueA)
	target.EXPECT().append(gomock.Any()).Return(nil)
	target.EXPECT().append(gomock.Any()).Return(issueB)
	target.EXPECT().close().Return(nil)

	for range 3 {
		sink.Append(&Entry{})
	}

	// Errors should be reported on Close.
	err := sink.Close()
	require.ErrorIs(t, err, issueA)
	require.ErrorIs(t, err, issueB)
}

func TestParquetSink_ErrorDuringAppend_CollectedErrorsSaturate(t *testing.T) {
	ctrl := gomock.NewController(t)
	target := NewMock_sinkTarget(ctrl)
	sink := startParquetSink(target)

	issue := fmt.Errorf("injected issue")
	target.EXPECT().append(gomock.Any()).Return(issue).AnyTimes()
	target.EXPECT().close().Return(nil)

	for range 50 {
		sink.Append(&Entry{})
	}

	type joined interface {
		Unwrap() []error
	}

	// Errors should be reported on Close.
	err := sink.Close()
	require.Error(t, err)
	issues := err.(joined).Unwrap()
	require.Len(t, issues, 11) // 10 issues + 1 suppression message
	require.Contains(t, issues[10].Error(), "suppressing further errors")
}

func TestParquetSink_ErrorDuringClose_IsAlwaysReported(t *testing.T) {
	ctrl := gomock.NewController(t)
	target := NewMock_sinkTarget(ctrl)
	sink := startParquetSink(target)

	issue := fmt.Errorf("injected append issue")
	closeIssue := fmt.Errorf("injected close issue")
	target.EXPECT().append(gomock.Any()).Return(issue).AnyTimes()
	target.EXPECT().close().Return(closeIssue)

	for range 50 {
		sink.Append(&Entry{})
	}

	type joined interface {
		Unwrap() []error
	}

	// Errors should be reported on Close.
	err := sink.Close()
	require.Error(t, err)
	issues := err.(joined).Unwrap()
	require.Len(t, issues, 12) // 10 issues + 1 suppression message + close issue
	require.Contains(t, issues[10].Error(), "suppressing further errors")
	require.ErrorIs(t, issues[11], closeIssue)
}

func TestParquetSink_Append_ClosedSink_HasNoEffect(t *testing.T) {
	ctrl := gomock.NewController(t)
	target := NewMock_sinkTarget(ctrl)
	sink := startParquetSink(target)

	target.EXPECT().close().Return(nil)

	require.NoError(t, sink.Close())
	sink.Append(&Entry{}) // should have no effect (no expected call on target)
}

func TestParquetSink_Close_ClosedSink_HasNoEffect(t *testing.T) {
	ctrl := gomock.NewController(t)
	target := NewMock_sinkTarget(ctrl)
	sink := startParquetSink(target)

	target.EXPECT().close().Return(nil)

	require.NoError(t, sink.Close())
	require.NoError(t, sink.Close()) // second close should have no effect
}

// --- ParquetExporter tests ---

func TestParquetExporter_OpenAndClose_CreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.parquet")
	require.False(t, exists(path))
	exporter, err := newParquetExporter(path, 100)
	require.NoError(t, err)
	require.NoError(t, exporter.close())
	require.True(t, exists(path))
}

func TestNewParquetExporter_FailsOnInvalidPath(t *testing.T) {
	invalidPath := filepath.Join(t.TempDir(), "nonexistent_dir", "test.parquet")
	_, err := newParquetExporter(invalidPath, 100)
	require.ErrorContains(t, err, "no such file or directory")
}

func TestNewParquetExporter_FailsIfFileWriterCreationFails(t *testing.T) {
	issue := errors.New("file writer creation failed")
	_, err := _newParquetExporter(
		filepath.Join(t.TempDir(), "test.parquet"),
		func(s *arrow.Schema, w io.Writer) (*pqarrow.FileWriter, error) {
			return nil, issue
		},
		100,
	)
	require.ErrorIs(t, err, issue)
}

func TestParquetExporter_CanAppendData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.parquet")
	exporter, err := newParquetExporter(path, 100)
	require.NoError(t, err)

	require.NoError(t, exporter.append(&Entry{
		Time: time.Unix(123456789, 0),
		Mark: mark.MsgSent,
	}))

	require.NoError(t, exporter.close())
	require.True(t, exists(path))

	// Note: This only tests that data was written without error. We currently
	// do not have infrastructure in Go to read back Parquet files for
	// verification. This is implicitly tested end-to-end by producing reports.
}

func TestParquetExporter_Append_CanHandleMetadataFields(t *testing.T) {
	tests := map[string]Metadata{
		"from_as_string":          toMeta("from", "alice"),
		"from_as_formattable":     toMeta("from", 12),
		"to_as_string":            toMeta("to", "bob"),
		"to_as_formattable":       toMeta("to", 34),
		"type_as_string":          toMeta("type", "transaction"),
		"type_as_formattable":     toMeta("type", 99),
		"sid_as_int":              toMeta("sid", 56),
		"sid_as_uint32":           toMeta("sid", uint32(78)),
		"id_as_int":               toMeta("id", 56),
		"id_as_uint32":            toMeta("id", uint32(78)),
		"NumNodes_as_int":         toMeta("NumNodes", 100),
		"NumNodes_as_uint32":      toMeta("NumNodes", uint32(200)),
		"TxPerSecond_as_int":      toMeta("TxPerSecond", 300),
		"TxPerSecond_as_uint32":   toMeta("TxPerSecond", uint32(400)),
		"hash_as_hash":            toMeta("hash", types.Hash{0x01, 0x02, 0x03}),
		"Topology_as_string":      toMeta("Topology", "fully-meshed"),
		"Topology_as_formattable": toMeta("Topology", mockTopology{name: "line-10"}),
	}

	for name, metadata := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.parquet")
			exporter, err := newParquetExporter(path, 100)
			require.NoError(t, err)

			entry := &Entry{
				Meta: metadata,
			}

			require.NoError(t, exporter.append(entry))
			require.NoError(t, exporter.close())
			require.True(t, exists(path))
		})
	}
}

// mockTopology is a mock implementation for testing topology metadata.
type mockTopology struct {
	name string
}

func (m mockTopology) String() string {
	return m.name
}

func TestParquetExporter_Append_InvalidMetadataType_ProducesAnError(t *testing.T) {
	tests := map[string]Metadata{
		"hash_as_unsupported_type":          toMeta("hash", "not_a_hash"),
		"NumValidators_as_unsupported_type": toMeta("NumValidators", "not_a_number"),
		"TxPerSecond_as_unsupported_type":   toMeta("TxPerSecond", "not_a_number"),
	}

	for name, metadata := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.parquet")
			exporter, err := newParquetExporter(path, 100)
			require.NoError(t, err)

			entry := &Entry{
				Meta: metadata,
			}

			err = exporter.append(entry)
			require.ErrorContains(t, err, "unsupported type for field")
			require.NoError(t, exporter.close())
		})
	}
}

func TestParquetExporter_Append_CloseExporter_ProducesAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.parquet")
	exporter, err := newParquetExporter(path, 100)
	require.NoError(t, err)

	require.NoError(t, exporter.close())
	err = exporter.append(&Entry{
		Time: time.Unix(123456789, 0),
		Mark: mark.MsgSent,
	})
	require.ErrorContains(t, err, "parquet exporter is closed")
}

func TestParquetExporter_ClosingAClosedExporterHasNoEffect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.parquet")
	exporter, err := newParquetExporter(path, 100)
	require.NoError(t, err)

	require.NoError(t, exporter.close())
	require.NoError(t, exporter.close())
}

func TestParquetExporter_CanCompressLargeAmountOfData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.parquet")
	exporter, err := newParquetExporter(path, 800)
	require.NoError(t, err)

	// Just appending the same data all the time should be very well compressible.
	// This also serves as a test for intermediate flushing and memory management.
	for range 1000 {
		require.NoError(t, exporter.append(&Entry{
			Time: time.Unix(123456789, 0),
			Mark: mark.MsgSent,
		}))
	}

	require.NoError(t, exporter.close())

	stats, err := os.Stat(path)
	require.NoError(t, err)
	require.Less(t, stats.Size(), int64(5*1<<10)) // less than 5 KB
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
