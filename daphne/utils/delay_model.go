package utils

import (
	"sync"
	"time"
)

// key can be either a string or an unsigned integer (or a type whose
// underlying type is one of those).
type key interface {
	~string | ~uint64 | ~uint32
}

// DelayModel is a delay model where all delays are time.Duration.
type DelayModel[T key, K any] interface {
	GetDelay(T, T) time.Duration
	ConfigureBaseDelay(config K)
	ConfigureCustomDelay(from, to T, config K)
}

// connectionKey stores two elements of the same type to represent a directed
// connection.
type connectionKey[T key] struct {
	from T
	to   T
}

// === FixedDelayModel ===

// FixedProcessingDelayModel is a fixed delay model with base and custom delays,
// supporting asymmetric per-connection delays.
type FixedDelayModel[T key] struct {
	baseDelay    time.Duration
	customDelays map[connectionKey[T]]time.Duration
	delaysMutex  sync.RWMutex
}

func NewFixedDelayModel[T key]() *FixedDelayModel[T] {
	return &FixedDelayModel[T]{
		customDelays: make(map[connectionKey[T]]time.Duration),
	}
}

func (m *FixedDelayModel[T]) ConfigureBaseDelay(delay time.Duration) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.baseDelay = delay
}

func (m *FixedDelayModel[T]) ConfigureCustomDelay(
	from, to T,
	delay time.Duration,
) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.customDelays[connectionKey[T]{from, to}] = delay
}

func (m *FixedDelayModel[T]) GetDelay(from, to T) time.Duration {
	m.delaysMutex.RLock()
	defer m.delaysMutex.RUnlock()
	if delay, exists := m.customDelays[connectionKey[T]{from, to}]; exists {
		return delay
	}
	return m.baseDelay
}

// === SampledDelayModel ===

// Distribution is the minimal interface a sampling distribution must satisfy.
// Any distribution type that can produce a positive duration implements this.
type Distribution interface {
	Sample() float64
	SampleDuration() time.Duration
}

// SampledDelayModel is a delay model that samples delays from distributions,
// providing more realistic delay simulations. Custom distributions can be set
// for specific connections asymmetrically.
type SampledDelayModel[T key] struct {
	baseDistribution    Distribution
	customDistributions map[connectionKey[T]]Distribution
	distributionsMutex  sync.RWMutex

	// timeUnit defines the unit for sampled delays (e.g., time.Millisecond).
	timeUnit time.Duration
}

func NewSampledDelayModel[T key]() *SampledDelayModel[T] {
	return &SampledDelayModel[T]{
		customDistributions: make(map[connectionKey[T]]Distribution),
	}
}

func (m *SampledDelayModel[T]) ConfigureBaseDelay(distribution Distribution) {
	m.distributionsMutex.Lock()
	defer m.distributionsMutex.Unlock()
	m.baseDistribution = distribution
}

func (m *SampledDelayModel[T]) ConfigureCustomDelay(
	from, to T,
	distribution Distribution,
) {
	m.distributionsMutex.Lock()
	defer m.distributionsMutex.Unlock()
	m.customDistributions[connectionKey[T]{from, to}] = distribution
}

func (m *SampledDelayModel[T]) GetDelay(from, to T) time.Duration {
	m.distributionsMutex.RLock()
	defer m.distributionsMutex.RUnlock()

	var dist Distribution
	if customDist, exists := m.customDistributions[connectionKey[T]{
		from,
		to,
	}]; exists {
		dist = customDist
	} else {
		dist = m.baseDistribution
	}

	if dist == nil {
		return 0
	}
	return dist.SampleDuration()
}

// --- LogNormalDistribution Helpers ---

// Convenience helpers for LogNormalDistribution.
func (m *SampledDelayModel[T]) SetBaseDistribution(
	mu, sigma float64,
	timeUnit time.Duration,
	seed *int64,
) {
	m.ConfigureBaseDelay(NewLogNormalDistribution(mu, sigma, timeUnit, seed))
}

func (m *SampledDelayModel[T]) SetCustomDistribution(
	from, to T,
	mu, sigma float64,
	timeUnit time.Duration,
	seed *int64,
) {
	m.ConfigureCustomDelay(
		from,
		to,
		NewLogNormalDistribution(mu, sigma, timeUnit, seed),
	)
}
