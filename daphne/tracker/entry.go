package tracker

import (
	"time"

	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
)

// Entry is a single tracked event with its associated metadata.
type Entry struct {
	Time time.Time
	Mark mark.Mark
	Meta Metadata
}
