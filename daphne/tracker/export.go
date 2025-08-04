package tracker

import (
	"encoding/csv"
	"fmt"
	"io"
	"maps"
	"slices"
)

//go:generate mockgen -source export.go -destination=export_mock.go -package=tracker

// ExportCSV exports all collected tracker data to a file in CSV format.
func (t *rootTracker) ExportCSV(out Writer) error {
	writer := csv.NewWriter(out)
	defer writer.Flush()

	// Collect and sort all keys from all entries.
	seen := make(map[string]struct{})
	for _, entry := range t.GetAll() {
		for key := range entry.Meta.data {
			seen[key] = struct{}{}
		}
	}
	// Convert keys to a sorted slice.
	keys := slices.Collect(maps.Keys(seen))
	slices.Sort(keys)

	header := []string{"Time", "Event"}
	header = append(header, keys...)

	// Write header
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write entries
	for _, entry := range t.GetAll() {
		values := make([]string, 0, len(header))
		values = append(values, fmt.Sprintf("%d", entry.Time.UnixNano()))
		values = append(values, entry.Event)
		for _, key := range keys {
			values = append(values, entry.Meta.Get(key))
		}
		if err := writer.Write(values); err != nil {
			return err
		}
	}

	return nil
}

// Writer is a wrapper for the io.Writer to support the generation of a mock.
type Writer interface {
	io.Writer
}
