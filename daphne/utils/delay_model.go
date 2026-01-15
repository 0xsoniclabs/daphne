package utils

import (
	"sync"
	"time"
)

//go:generate mockgen -source delay_model.go -destination=delay_model_mock.go -package=utils

type Distribution interface {
	SampleDuration() time.Duration
	Quantile(probability float64) time.Duration
}

// DelayModel is a generic delay model that stores a base value and
// optional custom values keyed by any comparable type K. The getDelay
// function converts the stored value type V into a time.Duration.
type DelayModel[K comparable] struct {
	baseValue    Distribution
	customValues map[K]Distribution
	mutex        sync.RWMutex
	getDelay     func(Distribution) time.Duration
}

// NewDelayModel constructs a DelayModel with the supplied conversion
// function that turns a value of type V into a time.Duration.
func NewDelayModel[K comparable]() *DelayModel[K] {
	return &DelayModel[K]{
		customValues: make(map[K]Distribution),
		getDelay: func(v Distribution) time.Duration {
			if v == nil {
				return 0
			}
			return v.SampleDuration()
		},
	}
}

// ConfigureBase sets the base value used when no custom value exists
// for a given key.
func (m *DelayModel[K]) ConfigureBase(value Distribution) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.baseValue = value
}

// GetBase returns the base value.
func (m *DelayModel[K]) GetBase() Distribution {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.baseValue
}

// ConfigureCustom sets a custom value for a specific key.
func (m *DelayModel[K]) ConfigureCustom(key K, value Distribution) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.customValues[key] = value
}

// GetDelay returns the delay for the given key. If no custom value is
// set for that key, it falls back to the base value.
func (m *DelayModel[K]) GetDelay(key K) time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if custom, exists := m.customValues[key]; exists {
		return m.getDelay(custom)
	}
	return m.getDelay(m.baseValue)
}
