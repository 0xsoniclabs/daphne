package generic

import (
	"math/rand"
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
//
// Additional Functions Available exposed by distuv.LogNormal:
//
// - Statistical Properties:
//   - Mean() - Returns the mean of the probability distribution
//   - Median() - Returns the median of the probability distribution
//   - Mode() - Returns the mode of the probability distribution
//   - Variance() - Returns the variance of the probability distribution
//   - StdDev() - Returns the standard deviation of the probability distribution
//   - Skewness() - Returns the skewness of the distribution
//   - ExKurtosis() - Returns the excess kurtosis of the distribution
//   - Entropy() - Returns the differential entropy of the distribution
//
// - Probability Functions:
//   - CDF(x) - Computes the value of the cumulative density function at x
//   - Prob(x) - Computes the value of the probability density function at x
//   - LogProb(x) - Computes the natural logarithm of the value of the
//     probability density function at x
//   - Survival(x) - Returns the survival function (complementary CDF) at x
//   - Quantile(p) - Returns the inverse of the cumulative probability
//     distribution
//
// - Utility Functions:
//   - NumParameters() - Returns the number of parameters in the distribution
//     (returns 2 for log-normal)
//   - Rand() - Returns a random sample drawn from the distribution
type LogNormalDistribution struct {
	dist distuv.LogNormal
}

// NewLogNormalDistribution creates a new log-normal delay model. If seed is
// nil, it uses the current timestamp as the random source.
func NewLogNormalDistribution(mu, sigma float64, seed *int64) *LogNormalDistribution {
	var src rand.Source
	if seed != nil {
		src = rand.NewSource(*seed)
	} else {
		src = rand.NewSource(time.Now().UnixNano())
	}

	return &LogNormalDistribution{
		dist: distuv.LogNormal{
			Mu:    mu,
			Sigma: sigma,
			Src:   rand.New(src),
		},
	}

}

// Sample returns a positive random float64 drawn from the log-normal
// distribution.
func (l *LogNormalDistribution) Sample() float64 {
	return l.dist.Rand()
}

// SampleDuration returns a sampled value scaled into a time.Duration.
// Example: m.SampleDuration(time.Millisecond) → a duration in ms.
//
// SampleDuration returns a positive time.Duration sampled from the log-normal
// distribution. Mathematically: the value is X = exp(μ + σ * Z), where
// Z ~ Normal(0,1). This yields a log-normal random variable X. The result is
// then scaled by the given unit (e.g., time.Millisecond) to produce a
// concrete delay duration.
//
// Example:
//
//	m := NewLogNormalDistribution(3.0, 0.5, nil)
//	d := m.SampleDuration(time.Millisecond)
//
// Here, d is approximately exp(3 + 0.5*Z) ms for some random Z ~ N(0,1).
func (l *LogNormalDistribution) SampleDuration(unit time.Duration) time.Duration {
	return time.Duration(l.Sample() * float64(unit))
}
