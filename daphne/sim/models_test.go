package sim

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetDefaultStateProcessingLatencyModel_HasFittingPercentiles(t *testing.T) {

	model := getDefaultStateProcessingLatencyModel()

	// Check percentiles for transaction delays. Theses tests are mainly to
	// make sure that the order of magnitude is correct, and the rough shape of
	// the expected distribution is met.
	latency := model.GetBaseTransactionDistribution()
	us := time.Microsecond
	require.InDelta(t, latency.Quantile(0.5), 97*us, float64(us))
	require.InDelta(t, latency.Quantile(0.9), 582*us, float64(us))
	require.InDelta(t, latency.Quantile(0.99), 2502*us, float64(10*us))

	latency = model.GetBaseBlockFinalizationDistribution()
	ns := time.Nanosecond
	require.InDelta(t, latency.Quantile(0.5), 57*ns, float64(ns))
	require.InDelta(t, latency.Quantile(0.9), 332*ns, float64(ns))
	require.InDelta(t, latency.Quantile(0.99), 1386*ns, float64(10*ns))
}
