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

func TestLogNormalDistribution_NewFromMedianAndPercentile_ValidationErrors(t *testing.T) {
	validMedian := 50 * time.Millisecond
	validPTarget := 200 * time.Millisecond
	validP := 0.95
	validUnit := time.Millisecond

	tests := map[string]struct {
		median   time.Duration
		p        float64
		pTarget  time.Duration
		timeUnit time.Duration
		errText  string
	}{
		"median is zero": {
			median: 0, p: validP, pTarget: validPTarget, timeUnit: validUnit,
			errText: "median must be positive",
		},
		"median is negative": {
			median: -10 * time.Millisecond, p: validP, pTarget: validPTarget, timeUnit: validUnit,
			errText: "median must be positive",
		},
		"pTarget is zero": {
			median: validMedian, p: validP, pTarget: 0, timeUnit: validUnit,
			errText: "pTarget must be positive",
		},
		"pTarget is negative": {
			median: validMedian, p: validP, pTarget: -100 * time.Millisecond, timeUnit: validUnit,
			errText: "pTarget must be positive",
		},
		"timeUnit is zero": {
			median: validMedian, p: validP, pTarget: validPTarget, timeUnit: 0,
			errText: "timeUnit must be positive",
		},
		"timeUnit is negative": {
			median: validMedian, p: validP, pTarget: validPTarget, timeUnit: -1 * time.Nanosecond,
			errText: "timeUnit must be positive",
		},
		"p is 0.5": {
			median: validMedian, p: 0.5, pTarget: validPTarget, timeUnit: validUnit,
			errText: "percentile p must be in the (0.5, 1.0) range",
		},
		"p is less than 0.5": {
			median: validMedian, p: 0.499, pTarget: validPTarget, timeUnit: validUnit,
			errText: "percentile p must be in the (0.5, 1.0) range",
		},
		"p is 1.0": {
			median: validMedian, p: 1.0, pTarget: validPTarget, timeUnit: validUnit,
			errText: "percentile p must be in the (0.5, 1.0) range",
		},
		"p is greater than 1.0": {
			median: validMedian, p: 1.01, pTarget: validPTarget, timeUnit: validUnit,
			errText: "percentile p must be in the (0.5, 1.0) range",
		},
		"pTarget is equal to median": {
			median: 50 * time.Millisecond, p: validP, pTarget: 50 * time.Millisecond, timeUnit: validUnit,
			errText: "must be greater than the median",
		},
		"pTarget is less than median": {
			median: 50 * time.Millisecond, p: validP, pTarget: 49 * time.Millisecond, timeUnit: validUnit,
			errText: "must be greater than the median",
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)
			dist, err := NewFromMedianAndPercentile(
				testCase.median,
				testCase.p,
				testCase.pTarget,
				testCase.timeUnit,
				nil,
			)
			require.Error(err, "Expected an error but got nil")
			require.Nil(dist, "Expected distribution to be nil on error")
			require.ErrorContains(err, testCase.errText, "Error message mismatch")
		})
	}
}

func TestLogNormalDistribution_NewFromMedianAndPercentile_CalculatesCorrectMuAndSigma(t *testing.T) {
	require := require.New(t)

	median := 100 * time.Millisecond
	p := 0.95
	pTarget := 500 * time.Millisecond
	unit := time.Millisecond

	// Manual Calculation
	// m = 100.0
	m := float64(median) / float64(unit)
	// x_p = 500.0
	x_p := float64(pTarget) / float64(unit)

	// mu = ln(m)
	expectedMu := math.Log(m) // ~4.605170
	// z_p = N(0,1).Quantile(0.95)
	z_p := distuv.UnitNormal.Quantile(p) // ~1.644854
	// sigma = (ln(x_p) - ln(m)) / z_p = ln(x_p / m) / z_p
	expectedSigma := math.Log(x_p/m) / z_p // ln(5) / z_p ~ 0.978470

	dist, err := NewFromMedianAndPercentile(median, p, pTarget, unit, nil)
	require.NoError(err)
	require.NotNil(dist)

	require.Equal(unit, dist.timeUnit)
	require.InDelta(expectedMu, dist.dist.Mu, 1e-6)
	require.InDelta(expectedSigma, dist.dist.Sigma, 1e-6)
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
	require := require.New(t)

	median := 50 * time.Millisecond
	pIn := 0.95
	pTarget := 200 * time.Millisecond
	unit := time.Millisecond

	dist, err := NewFromMedianAndPercentile(median, pIn, pTarget, unit, nil)
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
	percentilesToCheck := []float64{0.50, 0.75, 0.90, 0.95, 0.99}
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

	// Confirm the theoretical P50 and P95 match the inputs exactly
	expectedP50 := float64(median) / float64(unit)
	expectedP95 := float64(pTarget) / float64(unit)
	require.InDelta(expectedP50, dist.dist.Quantile(0.50), 1e-7)
	require.InDelta(expectedP95, dist.dist.Quantile(0.95), 1e-7)
	require.InDelta(median, dist.dist.Quantile(0.5)*float64(unit), 1e-7)
	require.InDelta(pTarget, dist.dist.Quantile(pIn)*float64(unit), 1e-7)
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
