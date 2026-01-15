package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFixedDelay_SampleDuration_ReturnsTheUnderlyingDuration(t *testing.T) {
	delay := 150 * time.Second

	for range 10 {
		require.EqualValues(t, delay, FixedDelay(delay).SampleDuration())
	}
}

func TestFixedDelay_Quantile_ReturnsTheUnderlyingDuration(t *testing.T) {
	delay := 42 * time.Minute

	for _, p := range []float64{0.0, 0.25, 0.5, 0.75, 1.0} {
		require.EqualValues(t, delay, FixedDelay(delay).Quantile(p))
	}
}
