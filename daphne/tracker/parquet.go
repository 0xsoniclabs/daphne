package tracker

import (
	"fmt"
	"os"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/compress"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

// Tools: (source: https://github.com/apache/arrow-go/blob/main/parquet/doc.go)
//  go install github.com/apache/arrow-go/v18/parquet/cmd/parquet_reader@latest
//  go install github.com/apache/arrow-go/v18/parquet/cmd/parquet_schema@latest

type parquetExporter struct {
	recordBuilder *array.RecordBuilder
	writer        *pqarrow.FileWriter
}

func newParquetExporter(path string) (*parquetExporter, error) {

	schema := arrow.NewSchema(
		[]arrow.Field{
			// -- mandatory fields --
			{Name: "timestamp", Type: arrow.PrimitiveTypes.Uint64, Nullable: false},
			{Name: "mark", Type: arrow.BinaryTypes.String, Nullable: false},
			// -- optional fields --
			{Name: "from", Type: arrow.BinaryTypes.String, Nullable: true},
			{Name: "to", Type: arrow.BinaryTypes.String, Nullable: true},
			{Name: "id", Type: arrow.PrimitiveTypes.Uint32, Nullable: true},
			{Name: "type", Type: arrow.BinaryTypes.String, Nullable: true},
			{Name: "sid", Type: arrow.PrimitiveTypes.Uint32, Nullable: true},
			{Name: "NumNodes", Type: arrow.PrimitiveTypes.Uint32, Nullable: true},
			{Name: "TxPerSecond", Type: arrow.PrimitiveTypes.Uint32, Nullable: true},
		},
		nil,
	)

	pool := memory.NewGoAllocator()
	builder := array.NewRecordBuilder(pool, schema)

	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	writerProperties := parquet.NewWriterProperties(
		parquet.WithCompression(compress.Codecs.Snappy),
	)
	dataProperties := pqarrow.DefaultWriterProps()

	out, err := pqarrow.NewFileWriter(schema, file, writerProperties, dataProperties)
	if err != nil {
		return nil, err
	}

	return &parquetExporter{
		recordBuilder: builder,
		writer:        out,
	}, nil
}

func (e *parquetExporter) append(entry *Entry) error {

	for i, field := range e.recordBuilder.Schema().Fields() {
		builder := e.recordBuilder.Field(i)
		switch field.Name {
		case "timestamp":
			builder.(*array.Uint64Builder).Append(uint64(entry.Time.UnixNano()))
		case "mark":
			builder.(*array.StringBuilder).Append(entry.Mark.String())
		case "from", "to", "type":
			builder := builder.(*array.StringBuilder)
			if v := entry.Meta.Get(field.Name); v != nil {
				switch v := v.(type) {
				case string:
					builder.Append(v)
				default:
					builder.Append(fmt.Sprintf("%v", v))
				}
			} else {
				builder.AppendNull()
			}
		case "sid", "id", "NumNodes", "TxPerSecond":
			builder := builder.(*array.Uint32Builder)
			if v := entry.Meta.Get(field.Name); v != nil {
				switch v := v.(type) {
				case int:
					builder.Append(uint32(v))
				case uint32:
					builder.Append(v)
				default:
					panic(fmt.Sprintf("unsupported type for field %s: %T", field.Name, v))
				}
			} else {
				builder.AppendNull()
			}
		}
	}

	if e.recordBuilder.Field(0).Len() >= 1_000_000 {
		e.flush()
	}

	return nil
}

func (e *parquetExporter) flush() error {
	if e.recordBuilder.Field(0).Len() == 0 {
		return nil
	}
	batch := e.recordBuilder.NewRecordBatch()
	defer batch.Release()
	return e.writer.Write(batch)
}

func (e *parquetExporter) close() error {
	if e.writer == nil {
		return nil
	}

	e.flush()
	e.recordBuilder.Release()
	err := e.writer.Close()
	e.writer = nil
	return err
}
