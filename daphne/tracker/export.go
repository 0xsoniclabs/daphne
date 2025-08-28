package tracker

import (
	"encoding/csv"
	"fmt"
	"io"
	"maps"
	"slices"
)

//go:generate mockgen -source export.go -destination=export_mock.go -package=tracker

// ExportAsCSV exports the provided data using the CSV format.
func ExportAsCSV(data []Entry, out _Writer) error {
	writer := csv.NewWriter(out)
	defer writer.Flush()

	// Collect and sort all keys from all entries.
	seen := make(map[string]struct{})
	for _, entry := range data {
		for _, key := range entry.Meta.Keys() {
			seen[key] = struct{}{}
		}
	}
	// Convert keys to a sorted slice.
	keys := slices.Collect(maps.Keys(seen))
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
