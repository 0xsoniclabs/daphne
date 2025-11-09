package utils

import (
	"sync"
	"time"
)

// Distribution is any type that can produce a sampled time.Duration.
type Distribution interface {
	SampleDuration() time.Duration
}

// DelayModel is a generic delay model that stores a base value and
// optional custom values keyed by any comparable type K. The getDelay
// function converts the stored value type V into a time.Duration.
type DelayModel[K comparable, V any] struct {
	baseValue    V
	customValues map[K]V
	mutex        sync.RWMutex
	getDelay     func(V) time.Duration
}

// NewDelayModel constructs a DelayModel with the supplied conversion
// function that turns a value of type V into a time.Duration.
func NewDelayModel[K comparable, V any](getDelay func(V) time.Duration) *DelayModel[K, V] {
	return &DelayModel[K, V]{
		customValues: make(map[K]V),
		getDelay:     getDelay,
	}
}

// ConfigureBase sets the base value used when no custom value exists
// for a given key.
func (m *DelayModel[K, V]) ConfigureBase(value V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.baseValue = value
}

// GetBase returns the base value.
func (m *DelayModel[K, V]) GetBase() V {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.baseValue
}

// ConfigureCustom sets a custom value for a specific key.
func (m *DelayModel[K, V]) ConfigureCustom(key K, value V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.customValues[key] = value
}

// GetDelay returns the delay for the given key. If no custom value is
// set for that key, it falls back to the base value.
func (m *DelayModel[K, V]) GetDelay(key K) time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if custom, exists := m.customValues[key]; exists {
		return m.getDelay(custom)
	}
	return m.getDelay(m.baseValue)
}

// NewFixedDelayModel creates a DelayModel where stored values are
// plain time.Duration values and where getDelay simply returns them.
func NewFixedDelayModel[K comparable]() *DelayModel[K, time.Duration] {
	return NewDelayModel[K](func(d time.Duration) time.Duration {
		return d
	})
}

// NewSampledDelayModel creates a DelayModel where stored values are
// Distributions and where getDelay function samples the distribution.
func NewSampledDelayModel[K comparable]() *DelayModel[K, Distribution] {
	return NewDelayModel[K](func(d Distribution) time.Duration {
		if d == nil {
			return 0
		}
		return d.SampleDuration()
	})
}
