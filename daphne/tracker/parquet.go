package tracker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/compress"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

//go:generate mockgen -source parquet.go -destination=parquet_mock.go -package=tracker

// This file implements a Tracker Sink that exports tracked events to a Parquet
// file. Parquet is a columnar storage format that is efficient for both storage
// and querying, making it suitable for large volumes of tracking data. It
// also supports various compression algorithms to reduce disk space usage.
//
// The implementation uses Apache Arrow's Go library for Parquet handling. This
// package provides additional tools for working with Parquet files, including command-line
// utilities for reading and inspecting Parquet files.
//
// References: https://github.com/apache/arrow-go/blob/main/parquet/doc.go
//  go install github.com/apache/arrow-go/v18/parquet/cmd/parquet_reader@latest
//  go install github.com/apache/arrow-go/v18/parquet/cmd/parquet_schema@latest

const (
	// Buffer size for the channel used by the parquet sink to forward entries
	// to an async goroutine writing them to disk.
	sinkChannelBufferSize = 1_000_000

	// Number of entries to buffer in memory before flushing to disk.
	sinkFlushInterval = sinkChannelBufferSize
)

// NewParquetSink creates a new Parquet Sink that writes tracked events to the
// specified file path. If the file already exists, it will be overwritten.
// The Append operation of the returned Sink is non-blocking and errors during
// writing are collected and reported when the Sink is closed.
// Accesses to the resulting sink are thread-safe.
func NewParquetSink(path string) (*parquetSink, error) {
	out, err := newParquetExporter(path, sinkFlushInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to create parquet exporter: %w", err)
	}
	return startParquetSink(out), nil
}

// parquetSink is a Tracker Sink that asynchronously exports tracked events to a
// Parquet file. Use [NewParquetSink] to create a new instance.
type parquetSink struct {
	entryChannel      chan<- *Entry
	entryChannelMutex sync.Mutex
	done              <-chan struct{}
	issues            *[]error
}

type _sinkTarget interface {
	append(entry *Entry) error
	close() error
}

func startParquetSink(out _sinkTarget) *parquetSink {
	entries := make(chan *Entry, sinkChannelBufferSize)
	done := make(chan struct{})
	issues := &[]error{}
	go func() {
		defer close(done)
		for entry := range entries {
			if err := out.append(entry); err != nil {
				if len(*issues) < 10 {
					*issues = append(*issues, err)
				} else if len(*issues) == 10 {
					*issues = append(*issues,
						fmt.Errorf("more than 10 issues occurred, suppressing further errors"),
					)
				}
			}
		}
		if err := out.close(); err != nil {
			*issues = append(*issues, err)
		}
	}()
	return &parquetSink{
		entryChannel: entries,
		done:         done,
		issues:       issues,
	}
}

func (p *parquetSink) Append(entry *Entry) {
	p.entryChannelMutex.Lock()
	defer p.entryChannelMutex.Unlock()
	if channel := p.entryChannel; channel != nil {
		channel <- entry
	}
}

func (p *parquetSink) Close() error {
	p.entryChannelMutex.Lock()
	channel := p.entryChannel
	p.entryChannel = nil
	p.entryChannelMutex.Unlock()
	if channel == nil {
		return nil
	}
	close(channel)
	<-p.done
	p.done = nil
	return errors.Join(*p.issues...)
}

// --- ParquetExporter implementation ---

// parquetExporter handles the low-level details of writing entries to a Parquet
// file using Apache Arrow's Go library. All calls are synchronous.
type parquetExporter struct {
	recordBuilder *array.RecordBuilder
	writer        *pqarrow.FileWriter
	flushInterval int
}

// newParquetExporter creates a new Parquet exporter that writes to the
// specified file path.
func newParquetExporter(
	path string,
	flushInterval int,
) (*parquetExporter, error) {
	return _newParquetExporter(path,
		func(s *arrow.Schema, w io.Writer) (*pqarrow.FileWriter, error) {
			writerProperties := parquet.NewWriterProperties(
				parquet.WithCompression(compress.Codecs.Snappy),
			)
			dataProperties := pqarrow.DefaultWriterProps()
			return pqarrow.NewFileWriter(s, w, writerProperties, dataProperties)
		},
		flushInterval,
	)
}

func _newParquetExporter(
	path string,
	newFileWriter func(*arrow.Schema, io.Writer) (*pqarrow.FileWriter, error),
	flushInterval int,
) (*parquetExporter, error) {
	// Define the Parquet schema for the tracked entries.
	// If new fields are added to Entry.Meta, they should be added here as well.
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
			{Name: "rid", Type: arrow.PrimitiveTypes.Uint32, Nullable: true},
			{Name: "NumNodes", Type: arrow.PrimitiveTypes.Uint32, Nullable: true},
			{Name: "TxPerSecond", Type: arrow.PrimitiveTypes.Uint32, Nullable: true},
			{Name: "hash", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: true},
		},
		nil,
	)

	pool := memory.NewGoAllocator()
	builder := array.NewRecordBuilder(pool, schema)

	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	out, err := newFileWriter(schema, file)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}

	return &parquetExporter{
		recordBuilder: builder,
		writer:        out,
		flushInterval: flushInterval,
	}, nil
}

// append adds a new tracked entry to the Parquet file. The output is buffered.
func (e *parquetExporter) append(entry *Entry) error {
	if e.recordBuilder == nil {
		return fmt.Errorf("parquet exporter is closed")
	}

	// Converts the Entry into Parquet format by populating the record builder.
	// If new metadata fields are added to Entry.Meta, they should be handled
	// here as well.
	var err error
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
		case "rid", "id", "NumNodes", "TxPerSecond":
			builder := builder.(*array.Uint32Builder)
			if v := entry.Meta.Get(field.Name); v != nil {
				switch v := v.(type) {
				case int:
					builder.Append(uint32(v))
				case uint32:
					builder.Append(v)
				default:
					builder.AppendNull()
					err = errors.Join(err, fmt.Errorf("unsupported type for field %s: %T", field.Name, v))
				}
			} else {
				builder.AppendNull()
			}
		case "hash":
			builder := builder.(*array.FixedSizeBinaryBuilder)
			if v := entry.Meta.Get(field.Name); v != nil {
				switch v := v.(type) {
				case types.Hash:
					builder.Append(v[:])
				default:
					builder.AppendNull()
					err = errors.Join(err, fmt.Errorf("unsupported type for field %s: %T", field.Name, v))
				}
			} else {
				builder.AppendNull()
			}
		}
	}

	if e.recordBuilder.Field(0).Len() >= e.flushInterval {
		err = errors.Join(err, e.flush())
	}

	return err
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

	flushError := e.flush()
	e.recordBuilder.Release()
	e.recordBuilder = nil
	closeError := e.writer.Close()
	e.writer = nil
	return errors.Join(flushError, closeError)
}
