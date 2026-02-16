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
