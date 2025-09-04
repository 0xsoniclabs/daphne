package sim

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApp_CanBeRunWithoutArguments(t *testing.T) {
	// A very basic smoke test.
	require.NoError(t, Run([]string{}))
}
