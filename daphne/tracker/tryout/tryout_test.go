package tryout

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/compress"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/stretchr/testify/require"
)

func TestProduceParquetFile(t *testing.T) {

	// Tools: (source: https://github.com/apache/arrow-go/blob/main/parquet/doc.go)
	//  go install github.com/apache/arrow-go/v18/parquet/cmd/parquet_reader@latest

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "timestamp", Type: arrow.PrimitiveTypes.Timestamp, Nullable: false},
			{Name: "mark", Type: arrow.BinaryTypes.String, Nullable: false},
		},
		nil,
	)

	//path := filepath.Join(t.TempDir(), "test.parquet")
	path := "./test.parquet"

	file, err := os.Create(path)
	require.NoError(t, err)

	properties := parquet.NewWriterProperties(
		parquet.WithCompression(compress.Codecs.Snappy),
	)

	out, err := pqarrow.NewFileWriter(schema, file, properties, pqarrow.DefaultWriterProps())
	require.NoError(t, err)

	mem := memory.NewGoAllocator()

	const NumBatches = 100_000
	const BatchSize = 1000
	for i := range NumBatches {
		fmt.Printf("Writing batch %d / %d\r", i+1, NumBatches)
		timestampBuilder := array.NewTimestampBuilder(mem, &arrow.TimestampType{Unit: arrow.Nanosecond})
		defer timestampBuilder.Release()

		markBuilder := array.NewStringBuilder(mem)
		defer markBuilder.Release()

		for j := range BatchSize {
			overallIndex := i*BatchSize + j
			time, err := arrow.TimestampFromTime(time.Now(), arrow.Nanosecond)
			require.NoError(t, err)
			timestampBuilder.Append(time)
			markBuilder.Append(fmt.Sprintf("mark_%d", overallIndex%5))
		}

		record := array.NewRecordBatch(schema, []arrow.Array{
			timestampBuilder.NewArray(),
			markBuilder.NewArray(),
		}, BatchSize)

		require.NoError(t, out.WriteBuffered(record))

		if i%1000 == 0 {
			fmt.Printf("Flushing at row %d\n", out.NumRows())
			out.NewBufferedRowGroup()
		}
	}

	require.NoError(t, out.Close())
}
