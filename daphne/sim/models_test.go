// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

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
