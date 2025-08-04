package autocracy

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
)

func TestAutocracy_IsALayeringImplementation(t *testing.T) {
	var _ layering.Layering = &Autocracy{}
}
