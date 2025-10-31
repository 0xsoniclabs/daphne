package utils

import (
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

func TestLogNormalDistribution_Sample_IsDeterministicWithSeed(t *testing.T) {
	require := require.New(t)

	seed := int64(12345)
	mu := 3.0
	sigma := 0.5

	// The expected sequence of samples based on the fixed seed and parameters.
	expectedSamples := []float64{
		35.48993969434574,
		17.733173068019678,
		25.35639457907359,
	}

	dist := NewLogNormalDistribution(mu, sigma, time.Millisecond, &seed)
	for _, expected := range expectedSamples {
		got := dist.Sample()
		require.InDelta(expected, got, 1e-9, "Sample() should produce a deterministic sequence")
	}
}

func TestLogNormalDistribution_SampleDuration_IsDeterministicWithSeed(t *testing.T) {
	require := require.New(t)

	seed := int64(12345)
	mu := 3.0
	sigma := 0.5

	// The expected sequence of samples based on the fixed seed and parameters.
	expectedSamples := []float64{
		35.48993969434574,
		17.733173068019678,
		25.35639457907359,
	}

	unit := time.Millisecond
	dist := NewLogNormalDistribution(mu, sigma, unit, &seed)
	for _, expectedSample := range expectedSamples {
		expectedDuration := time.Duration(expectedSample * float64(unit))
		gotDuration := dist.SampleDuration()
		require.Equal(expectedDuration, gotDuration, "SampleDuration() should produce a deterministic sequence")
	}
}

// Check if the sample statistics are "close enough" to the theoretical ones.
// We use a tolerance (ε) because random sampling won't be exact.
func TestLogNormalDistribution_Sample_HasCorrectStatisticalProperties(t *testing.T) {
	tests := map[string]struct {
		mu         float64
		sigma      float64
		meanTol    float64 // Relative tolerance for the mean
		varTol     float64 // Relative tolerance for the variance
		numSamples int
	}{
		"standard parameters": {
			mu:         2.0,
			sigma:      0.75,
			meanTol:    0.05, // 5%
			varTol:     0.1,  // 10%
			numSamples: 10_000_000,
		},
		"high variance": {
			mu:         1.0,
			sigma:      1.5,
			meanTol:    0.05, // 5%
			varTol:     0.25, // 25%
			numSamples: 10_000_000,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)

			unit := time.Millisecond
			// Use a random seed (nil) for statistical testing.
			l := NewLogNormalDistribution(testCase.mu, testCase.sigma, unit, nil)

			samples := make([]float64, testCase.numSamples)
			for i := range testCase.numSamples {
				samples[i] = l.Sample()
			}

			// Manually calculate theoretical mean and variance from the formulas.
			// Theoretical mean: E[X] = exp(μ + σ²/2)
			manualTheoreticalMean := math.Exp(testCase.mu + (testCase.sigma*testCase.sigma)/2)
			// Theoretical variance: Var[X] = (exp(σ²) - 1) * exp(2μ + σ²)
			manualTheoreticalVariance := (math.Exp(testCase.sigma*testCase.sigma) - 1) * math.Exp(2*testCase.mu+testCase.sigma*testCase.sigma)

			// Get the same values from the underlying package's functions.
			packageTheoreticalMean := l.dist.Mean()
			packageTheoreticalVariance := l.dist.Variance()

			// Assert that our manual calculation matches the package's calculation.
			require.InDelta(manualTheoreticalMean, packageTheoreticalMean, 1e-9, "Manual mean calculation should match package mean")
			require.InDelta(manualTheoreticalVariance, packageTheoreticalVariance, 1e-9, "Manual variance calculation should match package variance")

			// Calculate the actual mean and variance of the generated samples.
			sampleMean := stat.Mean(samples, nil)
			sampleVariance := stat.Variance(samples, nil)

			// Assert that the sample statistics are within a relative tolerance of the theoretical values.
			require.InDelta(packageTheoreticalMean, sampleMean, testCase.meanTol*packageTheoreticalMean, "Sample mean is outside tolerance")
			require.InDelta(packageTheoreticalVariance, sampleVariance, testCase.varTol*packageTheoreticalVariance, "Sample variance is outside tolerance")
		})
	}
}

func TestLogNormalDistribution_NewFromMedianAndPercentile_CalculatesCorrectMuAndSigma(t *testing.T) {
	unit := time.Millisecond

	tests := map[string]struct {
		median   time.Duration
		p        float64
		pTarget  time.Duration
		validate func(t *testing.T, mu, sigma float64)
	}{
		"p > 0.5 (P95)": {
			median:  100 * time.Millisecond,
			p:       0.95,
			pTarget: 500 * time.Millisecond,
			validate: func(t *testing.T, mu, sigma float64) {
				// Manual Calculation
				m := 100.0
				x_p := 500.0
				expectedMu := math.Log(m)               // ~4.605170
				z_p := distuv.UnitNormal.Quantile(0.95) // ~1.644854
				expectedSigma := math.Log(x_p/m) / z_p  // ln(5) / z_p ~ 0.978470
				require.InDelta(t, expectedMu, mu, 1e-6)
				require.InDelta(t, expectedSigma, sigma, 1e-6)
			},
		},
		"p < 0.5 (P10)": {
			median:  100 * time.Millisecond,
			p:       0.10,
			pTarget: 20 * time.Millisecond,
			validate: func(t *testing.T, mu, sigma float64) {
				// Manual Calculation
				m := 100.0
				x_p := 20.0
				expectedMu := math.Log(m)               // ~4.605170
				z_p := distuv.UnitNormal.Quantile(0.10) // ~ -1.28155
				expectedSigma := math.Log(x_p/m) / z_p  // ln(0.2) / z_p ~ 1.25585
				require.InDelta(t, expectedMu, mu, 1e-6)
				require.InDelta(t, expectedSigma, sigma, 1e-6)
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)

			dist, err := NewFromMedianAndPercentile(testCase.median, testCase.p, testCase.pTarget, unit, nil)
			require.NoError(err)
			require.NotNil(dist)
			require.Equal(unit, dist.timeUnit)

			testCase.validate(t, dist.dist.Mu, dist.dist.Sigma)
		})
	}
}

func TestLogNormalDistribution_NewFromMedianAndPercentile_CalculatesCorrectMuAndSigmaWithDifferentUnits(t *testing.T) {
	require := require.New(t)

	median := 50 * time.Millisecond // 50,000,000 ns
	p := 0.90
	pTarget := 1 * time.Second // 1,000,000,000 ns
	unit := time.Nanosecond

	// Manual Calculation - in nanoseconds
	// m = 50,000,000
	m := float64(median) / float64(unit)
	// x_p = 1,000,000,000
	x_p := float64(pTarget) / float64(unit)

	// mu = ln(50,000,000)
	expectedMu := math.Log(m) // ~17.72758
	// z_p = N(0,1).Quantile(0.90)
	z_p := distuv.UnitNormal.Quantile(p) // ~1.28155
	// sigma = ln(1,000,000,000 / 50,000,000) / z_p = ln(20) / z_p
	expectedSigma := math.Log(x_p/m) / z_p // ~2.33758

	dist, err := NewFromMedianAndPercentile(median, p, pTarget, unit, nil)
	require.NoError(err)
	require.NotNil(dist)

	require.Equal(unit, dist.timeUnit)
	require.InDelta(expectedMu, dist.dist.Mu, 1e-7)
	require.InDelta(expectedSigma, dist.dist.Sigma, 1e-7)
}

func TestLogNormalDistribution_NewFromMedianAndPercentile_ProducesCorrectPercentiles(t *testing.T) {
	unit := time.Millisecond

	tests := map[string]struct {
		median  time.Duration
		pIn     float64
		pTarget time.Duration
	}{
		"P95 > Median": {
			median:  50 * time.Millisecond,
			pIn:     0.95,
			pTarget: 200 * time.Millisecond,
		},
		"P10 < Median": {
			median:  50 * time.Millisecond,
			pIn:     0.10,
			pTarget: 10 * time.Millisecond,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)

			dist, err := NewFromMedianAndPercentile(testCase.median, testCase.pIn, testCase.pTarget, unit, nil)
			require.NoError(err)
			require.NotNil(dist)

			// Generate a large number of samples
			numSamples := 10_000_000
			samples := make([]float64, numSamples)
			for i := range numSamples {
				samples[i] = dist.Sample()
			}

			// Sort the samples, which is required for stat.Quantile with stat.Empirical
			sort.Float64s(samples)

			// Calculate weights for gonum stat.Quantile
			weights := make([]float64, numSamples)
			for i := range weights {
				weights[i] = 1.0 / float64(numSamples)
			}

			// Percentiles to check
			percentilesToCheck := []float64{0.10, 0.50, 0.75, 0.90, 0.95, 0.99}
			tolerance := 0.01

			for _, p := range percentilesToCheck {
				// Get the THEORETICAL quantile from the distribution we created
				expectedQuantile := dist.dist.Quantile(p)

				// Get the ACTUAL quantile from our 10M samples
				sampleQuantile := stat.Quantile(p, stat.Empirical, samples, weights)

				// Check that the sample statistic is within tolerance of the theoretical value
				msg := fmt.Sprintf("Sample P%.f (%.4f) is out of tolerance of theoretical P%.f (%.4f)",
					p*100, sampleQuantile, p*100, expectedQuantile)
				require.InDelta(expectedQuantile, sampleQuantile, expectedQuantile*tolerance, msg)
			}

			// Confirm the theoretical P50 and P_in match the inputs exactly
			expectedP50 := float64(testCase.median) / float64(unit)
			expectedP_in := float64(testCase.pTarget) / float64(unit)
			require.InDelta(expectedP50, dist.dist.Quantile(0.50), 1e-7)
			require.InDelta(expectedP_in, dist.dist.Quantile(testCase.pIn), 1e-7)
		})
	}
}

func TestLogNormalDistribution_NewFromMedianAndPercentile_IsDeterministicWithSeed(t *testing.T) {
	require := require.New(t)
	seed := int64(98765)

	median := 10 * time.Millisecond
	p := 0.8
	pTarget := 30 * time.Millisecond
	unit := time.Microsecond

	dist1, err1 := NewFromMedianAndPercentile(median, p, pTarget, unit, &seed)
	require.NoError(err1)

	dist2, err2 := NewFromMedianAndPercentile(median, p, pTarget, unit, &seed)
	require.NoError(err2)

	for range 10_000 {
		s1 := dist1.Sample()
		s2 := dist2.Sample()
		require.Equal(s1, s2, "Samples from distributions with the same seed should be identical")
	}
}

func TestLogNormalDistribution_NewFromTwoPercentiles_ValidationErrors(t *testing.T) {
	validP1 := 0.1
	validP1Target := 20 * time.Millisecond
	validP2 := 0.9
	validP2Target := 200 * time.Millisecond
	validUnit := time.Millisecond

	tests := map[string]struct {
		p1       float64
		p1Target time.Duration
		p2       float64
		p2Target time.Duration
		timeUnit time.Duration
		errText  string
	}{
		"p1Target is zero": {
			p1: validP1, p1Target: 0, p2: validP2, p2Target: validP2Target, timeUnit: validUnit,
			errText: "p1Target must be positive",
		},
		"p2Target is zero": {
			p1: validP1, p1Target: validP1Target, p2: validP2, p2Target: 0, timeUnit: validUnit,
			errText: "p2Target must be positive",
		},
		"timeUnit is zero": {
			p1: validP1, p1Target: validP1Target, p2: validP2, p2Target: validP2Target, timeUnit: 0,
			errText: "timeUnit must be positive",
		},
		"p1 is zero": {
			p1: 0, p1Target: validP1Target, p2: validP2, p2Target: validP2Target, timeUnit: validUnit,
			errText: "percentiles must be in the (0, 1) range",
		},
		"p1 is 1.0": {
			p1: 1.0, p1Target: validP1Target, p2: validP2, p2Target: validP2Target, timeUnit: validUnit,
			errText: "percentiles must be in the (0, 1) range",
		},
		"p2 is zero": {
			p1: validP1, p1Target: validP1Target, p2: 0, p2Target: validP2Target, timeUnit: validUnit,
			errText: "percentiles must be in the (0, 1) range",
		},
		"p2 is 1.0": {
			p1: validP1, p1Target: validP1Target, p2: 1.0, p2Target: validP2Target, timeUnit: validUnit,
			errText: "percentiles must be in the (0, 1) range",
		},
		"p1 equals p2": {
			p1: 0.5, p1Target: validP1Target, p2: 0.5, p2Target: validP2Target, timeUnit: validUnit,
			errText: "percentiles p1 and p2 must be different",
		},
		"p1Target equals p2Target": {
			p1: validP1, p1Target: 100 * time.Millisecond, p2: validP2, p2Target: 100 * time.Millisecond, timeUnit: validUnit,
			errText: "p1Target and p2Target must be different",
		},
		"contradiction: p2 > p1 but p2Target < p1Target": {
			p1: 0.1, p1Target: 200 * time.Millisecond, p2: 0.9, p2Target: 20 * time.Millisecond, timeUnit: validUnit,
			errText: "contradiction",
		},
		"contradiction: p1 > p2 but p1Target < p2Target": {
			p1: 0.9, p1Target: 20 * time.Millisecond, p2: 0.1, p2Target: 200 * time.Millisecond, timeUnit: validUnit,
			errText: "contradiction",
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)
			dist, err := NewFromTwoPercentiles(
				testCase.p1,
				testCase.p1Target,
				testCase.p2,
				testCase.p2Target,
				testCase.timeUnit,
				nil,
			)
			require.Error(err, "Expected an error but got nil")
			require.Nil(dist, "Expected distribution to be nil on error")
			require.ErrorContains(err, testCase.errText, "Error message mismatch")
		})
	}
}

func TestLogNormalDistribution_NewFromTwoPercentiles_CalculatesCorrectMuAndSigma(t *testing.T) {
	require := require.New(t)

	p1 := 0.10
	p1Target := 20 * time.Millisecond
	p2 := 0.90
	p2Target := 180 * time.Millisecond
	unit := time.Millisecond

	// Manual Calculation
	// x_p1 = 20.0
	x_p1 := float64(p1Target) / float64(unit)
	// x_p2 = 180.0
	x_p2 := float64(p2Target) / float64(unit)

	// z_p1 = N(0,1).Quantile(0.10)
	z_p1 := distuv.UnitNormal.Quantile(p1) // ~ -1.28155
	// z_p2 = N(0,1).Quantile(0.90)
	z_p2 := distuv.UnitNormal.Quantile(p2) // ~ +1.28155

	// sigma = ln(x_p2 / x_p1) / (z_p2 - z_p1)
	// sigma = ln(180 / 20) / (1.28155 - (-1.28155))
	// sigma = ln(9) / (2 * 1.28155)
	expectedSigma := math.Log(x_p2/x_p1) / (z_p2 - z_p1) // ~0.85735
	// mu = ln(x_p1) - (sigma * z_p1)
	// mu = ln(20) - (0.85735 * -1.28155)
	expectedMu := math.Log(x_p1) - (expectedSigma * z_p1) // ~4.09434

	dist, err := NewFromTwoPercentiles(p1, p1Target, p2, p2Target, unit, nil)
	require.NoError(err)
	require.NotNil(dist)

	require.Equal(unit, dist.timeUnit)
	require.InDelta(expectedMu, dist.dist.Mu, 1e-6)
	require.InDelta(expectedSigma, dist.dist.Sigma, 1e-6)

	// The median (P50) should be exp(mu)
	// median = exp(4.09434) = 60
	// Check the theoretical P50 of the resulting distribution
	expectedMedian := math.Exp(expectedMu)
	require.InDelta(expectedMedian, dist.dist.Quantile(0.50), 1e-6)
	require.InDelta(60.0, expectedMedian, 1e-6)
}

func TestLogNormalDistribution_NewFromTwoPercentiles_CalculatesCorrectMuAndSigmaWithDifferentUnits(t *testing.T) {
	require := require.New(t)

	p1 := 0.25
	p1Target := 500 * time.Microsecond // 500,000 ns
	p2 := 0.75
	p2Target := 2 * time.Millisecond // 2,000,000 ns
	unit := time.Nanosecond

	// Manual Calculation - in nanoseconds
	// x_p1 = 500,000
	x_p1 := float64(p1Target) / float64(unit)
	// x_p2 = 2,000,000
	x_p2 := float64(p2Target) / float64(unit)

	// z_p1 = N(0,1).Quantile(0.25)
	z_p1 := distuv.UnitNormal.Quantile(p1) // ~ -0.67449
	// z_p2 = N(0,1).Quantile(0.75)
	z_p2 := distuv.UnitNormal.Quantile(p2) // ~ +0.67449

	// sigma = ln(x_p2 / x_p1) / (z_p2 - z_p1)
	// sigma = ln(2,000,000 / 500,000) / (0.67449 - (-0.67449))
	// sigma = ln(4) / (2 * 0.67449)
	expectedSigma := math.Log(x_p2/x_p1) / (z_p2 - z_p1) // ~1.02796
	// mu = ln(x_p1) - (sigma * z_p1)
	// mu = ln(500,000) - (1.02796 * -0.67449)
	expectedMu := math.Log(x_p1) - (expectedSigma * z_p1) // ~13.81551

	dist, err := NewFromTwoPercentiles(p1, p1Target, p2, p2Target, unit, nil)
	require.NoError(err)
	require.NotNil(dist)

	require.Equal(unit, dist.timeUnit)
	require.InDelta(expectedMu, dist.dist.Mu, 1e-6)
	require.InDelta(expectedSigma, dist.dist.Sigma, 1e-6)

	// The median (P50) should be exp(mu)
	// P50 = exp(13.81551) = 1,000,000 ns = 1ms
	expectedMedian := math.Exp(expectedMu)
	require.InDelta(expectedMedian, dist.dist.Quantile(0.50), 1e-6)
	require.InDelta(1_000_000.0, expectedMedian, 1e-6)
}

func TestLogNormalDistribution_NewFromTwoPercentiles_ProducesCorrectPercentiles(t *testing.T) {
	require := require.New(t)

	p1 := 0.10
	p1Target := 20 * time.Millisecond
	p2 := 0.90
	p2Target := 180 * time.Millisecond
	unit := time.Millisecond

	dist, err := NewFromTwoPercentiles(p1, p1Target, p2, p2Target, unit, nil)
	require.NoError(err)
	require.NotNil(dist)

	// Generate a large number of samples
	numSamples := 10_000_000
	samples := make([]float64, numSamples)
	for i := range numSamples {
		samples[i] = dist.Sample()
	}

	// Sort the samples, which is required for stat.Quantile with stat.Empirical
	sort.Float64s(samples)

	// Calculate weights for gonum stat.Quantile
	weights := make([]float64, numSamples)
	for i := range weights {
		weights[i] = 1.0 / float64(numSamples)
	}

	// Percentiles to check
	percentilesToCheck := []float64{p1, 0.25, 0.50, 0.75, p2}
	tolerance := 0.01

	for _, p := range percentilesToCheck {
		// Get the THEORETICAL quantile from the distribution we created
		expectedQuantile := dist.dist.Quantile(p)

		// Get the ACTUAL quantile from our 10M samples
		sampleQuantile := stat.Quantile(p, stat.Empirical, samples, weights)

		// Check that the sample statistic is within tolerance of the theoretical value
		msg := fmt.Sprintf("Sample P%.f (%.4f) is out of tolerance of theoretical P%.f (%.4f)",
			p*100, sampleQuantile, p*100, expectedQuantile)
		require.InDelta(expectedQuantile, sampleQuantile, expectedQuantile*tolerance, msg)
	}

	// Confirm the theoretical P10 and P90 match the inputs exactly
	expectedP10 := float64(p1Target) / float64(unit)
	expectedP90 := float64(p2Target) / float64(unit)
	require.InDelta(expectedP10, dist.dist.Quantile(p1), 1e-7)
	require.InDelta(expectedP90, dist.dist.Quantile(p2), 1e-7)
	require.InDelta(float64(p1Target), dist.dist.Quantile(p1)*float64(unit), 1e-7)
	require.InDelta(float64(p2Target), dist.dist.Quantile(p2)*float64(unit), 1e-7)
}

func TestLogNormalDistribution_NewFromTwoPercentiles_IsDeterministicWithSeed(t *testing.T) {
	require := require.New(t)
	seed := int64(11111)

	p1 := 0.2
	p1Target := 100 * time.Millisecond
	p2 := 0.8
	p2Target := 300 * time.Millisecond
	unit := time.Microsecond

	dist1, err1 := NewFromTwoPercentiles(p1, p1Target, p2, p2Target, unit, &seed)
	require.NoError(err1)

	dist2, err2 := NewFromTwoPercentiles(p1, p1Target, p2, p2Target, unit, &seed)
	require.NoError(err2)

	for range 10_000 {
		s1 := dist1.Sample()
		s2 := dist2.Sample()
		require.Equal(s1, s2, "Samples from distributions with the same seed should be identical")
	}
}

func TestLogNormalDistribution_Sample_ProducesPositiveValues(t *testing.T) {
	require := require.New(t)

	unit := time.Millisecond
	dist := NewLogNormalDistribution(5.0, 2.0, unit, nil)

	for range 1000 {
		sample := dist.Sample()
		require.Greater(sample, float64(0), "Invariant failed: received a non-positive sample")
	}
}

func TestLogNormalDistribution_SampleDuration_ProducesPositiveValues(t *testing.T) {
	require := require.New(t)

	unit := time.Millisecond
	dist := NewLogNormalDistribution(5.0, 2.0, unit, nil)

	for range 1000 {
		durationSample := dist.SampleDuration()
		require.Greater(durationSample, time.Duration(0), "Invariant failed: received a non-positive duration")
	}
}
