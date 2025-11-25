package utils

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"gonum.org/v1/gonum/stat/distuv"
)

// LogNormalDistribution produces samples from a log-normal distribution.
//
// Background:
//   - A log-normal distribution is the distribution of e^X where
//     X ~ Normal(μ, σ). This means: take a random variable X from a
//     normal distribution with mean = μ and standard deviation = σ,
//     then exponentiate it. The result is always positive.
//   - This means sampled values are always positive and skewed to the right.
//   - It's useful for modeling real-world latencies and delays, which cannot
//     be negative and often have occasional "long-tail" high values.
//   - "Long-tail": Unlike a symmetric bell curve, a log-normal distribution
//     has a heavy right tail. Most samples cluster around smaller values, but
//     occasionally you get very large values. This models the fact that
//     network or processing delays are usually short, but sometimes
//     unexpectedly long.
//
// Parameters:
//   - μ (mu): the mean of the underlying normal distribution (before
//     exponentiation).
//   - σ (sigma): the standard deviation of the underlying normal distribution.
//   - Important: μ and σ do not directly correspond to the mean and std
//     dev of the samples of the distribution themselves. Instead, they
//     influence them indirectly. Larger μ shifts the whole distribution to
//     the right (increasing the typical delay), while larger σ increases
//     the spread and the likelihood of very large "long-tail" samples
//     (delays).
//   - Median: The median of the distribution is exactly exp(μ).
//   - Mean (E[X]): The mean is exp(μ + σ²/2). Note that the mean is
//     always greater than the median.
//   - Variance (Var[X]): The variance is (exp(σ²) - 1) * exp(2μ + σ²).
//     A larger σ drastically increases the variance and the "long-tail"
//     effect.
//
// --- Derivation of Formulas - Part One ---
//
// Our first goal is to find the μ (mu) and σ (sigma) parameters for the
// underlying normal distribution, given two percentile points:
//  1. The median (m), which is the 50th percentile (p=0.50).
//  2. A target percentile (x_p) at a given percentile p (e.g., p=0.95 or
//     p=0.10).
//
// We use two key relationships:
//
//  1. The Log-Normal to Normal Relationship:
//
//     A log-normal distribution is defined by its construction from a
//     normal distribution. A variable X is log-normal if it is the
//     exponential of a normally distributed variable Y.
//
//     - Start with a normal variable: Y ~ Normal(μ, σ)
//     - Create the log-normal variable: X = exp(Y)
//
//     To find the underlying normal variable Y from our log-normal
//     variable X, we just invert this definition by taking the
//     natural logarithm (ln) of both sides:
//
//     ln(X) = ln(exp(Y))
//     ln(X) = Y
//
//     This gives us our first key insight: any percentile of X (let's
//     call it x_p) can be mapped to the corresponding percentile of Y (y_p)
//     by taking its natural log:
//
//     y_p = ln(x_p)
//
//  2. The Normal to "Standard" Normal (Z-Score) Relationship:
//
//     Any percentile y_p from our specific normal distribution Y ~ Normal(μ, σ)
//     can be related back to the standard normal distribution Z ~ Normal(0, 1).
//     The p-th percentile of Z is called the Z-score, z_p.
//     (This z_p value is what distuv.UnitNormal.Quantile(p) gives us).
//
//     The formula to convert from the Z-score (z_p) to our percentile (y_p) is:
//
//     y_p = μ + (σ * z_p)
//
// By combining (1) and (2), we get our main equation:
//
//	ln(x_p) = μ + (σ * z_p)
//
// --- Derivation of Formulas - Part Two ---
//
// Our second goal is to find the μ (mu) and σ (sigma) parameters for the
// underlying normal distribution, given two percentile points:
//
//  1. (p1, x_p1): The target value x_p1 for the p1 percentile
//     (e.g., p1=0.1, x_p1=20ms)
//
//  2. (p2, x_p2): The target value x_p2 for the p2 percentile
//     (e.g., p2=0.9, x_p2=150ms)
//
// We use our main equation:
//
//	ln(x_p) = μ + (σ * z_p)
//
// This gives us a system of two linear equations with two unknowns (μ and σ):
//
//  1. ln(x_p1) = μ + (σ * z_p1)
//  2. ln(x_p2) = μ + (σ * z_p2)
//
// Step 1: Solve for σ (sigma) by subtracting equation (1) from (2)
//
//   - [ln(x_p2) = μ + (σ * z_p2)]
//
//   - [ln(x_p1) = μ + (σ * z_p1)]
//     ------------------------------------
//     ln(x_p2) - ln(x_p1) = (μ - μ) + (σ * z_p2) - (σ * z_p1)
//
//   - Simplify the terms:
//     ln(x_p2 / x_p1) = σ * (z_p2 - z_p1)
//
//   - Isolate σ:
//     σ = ln(x_p2 / x_p1) / (z_p2 - z_p1)
//
// Step 2: Solve for μ (mu) by substituting σ back into equation (1)
//
//   - We know from (1) that:
//     μ = ln(x_p1) - (σ * z_p1)
//
//   - With our value for σ from Step 1, we now have both parameters.
//
// Units:
//   - The values sampled from the distribution are raw float64 numbers.
//   - They must be scaled to a concrete time unit (e.g., milliseconds).
//
// Example:
//
//	m := NewLogNormalDistribution(3.0, 0.5, nil)
//	d := m.SampleDuration(time.Millisecond) // returns ~exp(N(3,0.5^2)) ms
//
// Note on "~exp(N(3,0.5^2))":
//   - This shorthand means "a sample drawn from the exponential of a normal
//     distribution with mean = 3 and variance = 0.25 [σ^2 = 0.5^2]".
//   - Mathematically: X = exp(μ + σ * Z), where Z ~ Normal(0,1).
//   - So with μ = 3.0 and σ = 0.5, X = exp(3 + 0.5*Z).
//   - For example, if Z = 0.2, then X ≈ exp(3.1) ≈ 22.2.
//   - If unit = time.Millisecond, then SampleDuration returns ≈ 22.2 ms.
type LogNormalDistribution struct {
	Dist     distuv.LogNormal
	timeUnit time.Duration
	mu       sync.Mutex
}

// NewLogNormalDistribution creates a new log-normal delay model. If seed is
// nil, it uses the current timestamp as the random source.
func NewLogNormalDistribution(
	mu, sigma float64,
	timeUnit time.Duration,
	seed *int64,
) *LogNormalDistribution {
	var src rand.Source
	if seed != nil {
		src = rand.NewSource(*seed)
	} else {
		src = rand.NewSource(time.Now().UnixNano())
	}

	return &LogNormalDistribution{
		Dist: distuv.LogNormal{
			Mu:    mu,
			Sigma: sigma,
			Src:   rand.New(src),
		},
		timeUnit: timeUnit,
	}
}

// NewFromMedianAndPercentile creates a new log-normal distribution by
// specifying its median (P50) and one other percentile (e.g., P95 or P10).
//
// This is often more intuitive than specifying μ (mu) and σ (sigma) directly,
// as it allows you to define the distribution based on its observable behavior
// (e.g., "the median delay is 50ms, and P95 is 200ms").
//
// Parameters:
//   - median: The desired 50th percentile (e.g., 50ms). Must be > 0.
//   - p: The percentile to specify (e.g., 0.95 or 0.10). Must be in the
//     range (0, 1) and cannot be 0.5.
//   - pTarget: The target duration for that percentile. Must be > median if
//     p > 0.5, or < median if p < 0.5.
//   - timeUnit: The base unit for the distribution (e.g., time.Millisecond).
//   - seed: Optional random seed.
//
// Returns:
//   - A configured *LogNormalDistribution.
//   - An error if parameters are invalid.
func NewFromMedianAndPercentile(
	median time.Duration,
	p float64,
	pTarget time.Duration,
	timeUnit time.Duration,
	seed *int64,
) (*LogNormalDistribution, error) {
	return NewFromTwoPercentiles(0.5, median, p, pTarget, timeUnit, seed)
}

// NewFromTwoPercentiles creates a new log-normal distribution by specifying
// two distinct percentile points (e.g., P10 and P90).
//
// This is another intuitive way to define the distribution based on its
// observable behavior (e.g., "10% of delays are under 20ms, and 90% are
// under 150ms [or 10% are above 150ms]").
//
// Parameters:
//   - p1: The first percentile (e.g., 0.10). Must be > 0, < 1.
//   - p1Target: The target duration for the p1 percentile. Must be > 0.
//   - p2: The second percentile (e.g., 0.90). Must be > 0, < 1, and != p1.
//   - p2Target: The target duration for the p2 percentile. Must be > 0,
//     and != p1Target.
//   - timeUnit: The base unit for the distribution (e.g., time.Millisecond).
//   - seed: Optional random seed.
//
// Returns:
//   - A configured *LogNormalDistribution.
//   - An error if parameters are invalid.
func NewFromTwoPercentiles(
	p1 float64,
	p1Target time.Duration,
	p2 float64,
	p2Target time.Duration,
	timeUnit time.Duration,
	seed *int64,
) (*LogNormalDistribution, error) {
	if p1Target <= 0 {
		return nil, errors.New("p1Target must be positive")
	}
	if p2Target <= 0 {
		return nil, errors.New("p2Target must be positive")
	}
	if timeUnit <= 0 {
		return nil, errors.New("timeUnit must be positive")
	}
	if p1 <= 0 || p1 >= 1 {
		return nil, fmt.Errorf("percentiles must be in the (0, 1) range, got %f", p1)
	}
	if p2 <= 0 || p2 >= 1 {
		return nil, fmt.Errorf("percentiles must be in the (0, 1) range, got %f", p2)
	}
	if p1 == p2 {
		return nil, errors.New("percentiles p1 and p2 must be different")
	}
	if p1Target == p2Target {
		return nil, errors.New("p1Target and p2Target must be different")
	}

	// Check for contradictory constraints (e.g., P90 < P10)
	if (p2 > p1) && (p2Target < p1Target) {
		return nil, fmt.Errorf(
			"contradiction: p2 (%f) > p1 (%f) but p2Target (%s) < p1Target (%s)",
			p2, p1, p2Target, p1Target,
		)
	}
	if (p1 > p2) && (p1Target < p2Target) {
		return nil, fmt.Errorf(
			"contradiction: p1 (%f) > p2 (%f) but p1Target (%s) < p2Target (%s)",
			p1, p2, p1Target, p2Target,
		)
	}

	// x_p1 is the target value for percentile p1
	x_p1 := float64(p1Target) / float64(timeUnit)
	// x_p2 is the target value for percentile p2
	x_p2 := float64(p2Target) / float64(timeUnit)

	// Get the Z-scores for each percentile
	z_p1 := distuv.UnitNormal.Quantile(p1)
	z_p2 := distuv.UnitNormal.Quantile(p2)

	// σ = ln(x_p2 / x_p1) / (z_p2 - z_p1)
	sigma := math.Log(x_p2/x_p1) / (z_p2 - z_p1)

	// μ = ln(x_p1) - (σ * z_p1)
	mu := math.Log(x_p1) - (sigma * z_p1)

	return NewLogNormalDistribution(mu, sigma, timeUnit, seed), nil
}

// Sample returns a positive random float64 drawn from the log-normal
// distribution.
func (l *LogNormalDistribution) Sample() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Dist.Rand()
}

// SampleDuration returns a sampled value scaled into a time.Duration.
// Example: m.SampleDuration() → a duration in ms.
//
// SampleDuration returns a positive time.Duration sampled from the log-normal
// distribution. Mathematically: the value is X = exp(μ + σ * Z), where
// Z ~ Normal(0,1). This yields a log-normal random variable X. The result is
// then scaled by the given unit (e.g., time.Millisecond) to produce a
// concrete delay duration.
//
// Example:
//
//	m := NewLogNormalDistribution(3.0, 0.5, time.Millisecond, nil)
//	d := m.SampleDuration()
//
// Here, d is approximately exp(3 + 0.5*Z) ms for some random Z ~ N(0,1).
func (l *LogNormalDistribution) SampleDuration() time.Duration {
	return time.Duration(l.Sample() * float64(l.timeUnit))
}

// Quantile returns the value below which a given percentage of samples fall.
//
// For example, Quantile(0.95) returns the value below which 95% of samples
// lie (the 95th percentile).
//
// Parameters:
//   - probability: A float64 in the range [0, 1] representing the desired
//     percentile (e.g., 0.5 for median, 0.95 for 95th percentile).
//
// The function panics if probability is outside the [0, 1] range.
func (l *LogNormalDistribution) Quantile(probability float64) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	value := l.Dist.Quantile(probability)
	return time.Duration(value * float64(l.timeUnit))
}
