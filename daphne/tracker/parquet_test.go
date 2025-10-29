package tracker

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParquetExport_OpenAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.parquet")
	exporter, err := newParquetExporter(path)
	require.NoError(t, err)
	require.NoError(t, exporter.close())
}
