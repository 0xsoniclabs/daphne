package tracker

import (
	"time"
)

// Entry is a single tracked event with its associated metadata.
type Entry struct {
	Time  time.Time
	Event string
	Meta  Metadata
}
