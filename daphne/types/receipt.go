package types

import (
	"fmt"
)

// Receipt represents the outcome of a Daphne transaction execution.
type Receipt struct {
	Success bool
}

func (r *Receipt) String() string {
	return fmt.Sprintf("{Success:%t}", r.Success)
}
