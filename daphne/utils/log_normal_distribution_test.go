package utils

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
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

	dist := NewLogNormalDistribution(mu, sigma, &seed)
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

	dist := NewLogNormalDistribution(mu, sigma, &seed)
	unit := time.Millisecond
	for _, expectedSample := range expectedSamples {
		expectedDuration := time.Duration(expectedSample * float64(unit))
		gotDuration := dist.SampleDuration(unit)
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
			meanTol:    0.05,
			varTol:     0.15,
			numSamples: 10_000_000,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)

			// Use a random seed (nil) for statistical testing.
			l := NewLogNormalDistribution(testCase.mu, testCase.sigma, nil)

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

func TestLogNormalDistribution_Sample_ProducesPositiveValues(t *testing.T) {
	require := require.New(t)
	dist := NewLogNormalDistribution(5.0, 2.0, nil)

	for range 1000 {
		sample := dist.Sample()
		require.Greater(sample, float64(0), "Invariant failed: received a non-positive sample")
	}
}

func TestLogNormalDistribution_SampleDuration_ProducesPositiveValues(t *testing.T) {
	require := require.New(t)
	dist := NewLogNormalDistribution(5.0, 2.0, nil)

	for range 1000 {
		durationSample := dist.SampleDuration(time.Second)
		require.Greater(durationSample, time.Duration(0), "Invariant failed: received a non-positive duration")
	}
}
