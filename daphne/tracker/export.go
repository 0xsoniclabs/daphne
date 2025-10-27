package tracker

import (
	"encoding/csv"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

//go:generate mockgen -source export.go -destination=export_mock.go -package=tracker

// ExportAsCSV exports the provided data using the CSV format.
func ExportAsCSV(data []Entry, out _Writer) error {
	writer := csv.NewWriter(out)
	defer writer.Flush()

	// Collect and sort all keys from all entries.
	seen := sets.Empty[string]()
	for _, entry := range data {
		for _, key := range entry.Meta.Keys() {
			seen.Add(key)
		}
	}
	// Convert keys to a sorted slice.
	keys := seen.ToSlice()
	slices.Sort(keys)

	header := []string{"timestamp", "mark"}
	header = append(header, keys...)

	// Write header
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write entries
	for _, entry := range data {
		values := make([]string, 0, len(header))
		values = append(values, fmt.Sprintf("%d", entry.Time.UnixNano()))
		values = append(values, entry.Mark.String())
		for _, key := range keys {
			values = append(values, entry.Meta.Get(key))
		}
		if err := writer.Write(values); err != nil {
			return err
		}
	}

	return nil
}

// _Writer is a wrapper for the io.Writer to support the generation of a mock.
type _Writer interface {
	io.Writer
}

func ExportAsJson(data Entry, out io.Writer) error {
	builder := strings.Builder{}
	builder.WriteString("\n{")
	builder.WriteString("\"timestamp\":")
	builder.WriteString(fmt.Sprintf("%d", data.Time.UnixNano()))
	builder.WriteString(",\"mark\":\"")
	builder.WriteString(data.Mark.String())
	builder.WriteString("\"")

	for _, key := range data.Meta.Keys() {
		builder.WriteString(",\"")
		builder.WriteString(key)
		builder.WriteString("\":\"")
		builder.WriteString(data.Meta.Get(key))
		builder.WriteString("\"")
	}

	builder.WriteRune('}')
	_, err := out.Write([]byte(builder.String()))
	return err
}
