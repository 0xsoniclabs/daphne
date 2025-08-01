package layering

import "testing"

func TestAutocracy_IsALayeringImplementation(t *testing.T) {
	var _ Layering = &Autocracy{}
}
