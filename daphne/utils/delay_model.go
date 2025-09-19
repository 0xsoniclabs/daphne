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

// connectionKey stores two elements of the same type to represent a directed
// connection.
type connectionKey[T key] struct {
	from T
	to   T
}

type Distribution interface {
	Sample() float64
	SampleDuration() time.Duration
}

// DelayModel stores base and per-connection custom values of any type V.
// The getDelay function defines how to leverage V to turn into a time.Duration
// delay.
type DelayModel[T key, V any] struct {
	baseValue    V
	customValues map[connectionKey[T]]V
	mutex        sync.RWMutex
	getDelay     func(V) time.Duration
}

func NewDelayModel[T key, V any](getDelay func(V) time.Duration) *DelayModel[T, V] {
	return &DelayModel[T, V]{
		customValues: make(map[connectionKey[T]]V),
		getDelay:     getDelay,
	}
}

func (m *DelayModel[T, V]) ConfigureBase(value V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.baseValue = value
}

func (m *DelayModel[T, V]) ConfigureCustom(from, to T, value V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.customValues[connectionKey[T]{from, to}] = value
}

func (m *DelayModel[T, V]) GetDelay(from, to T) time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if custom, exists := m.customValues[connectionKey[T]{from, to}]; exists {
		return m.getDelay(custom)
	}
	return m.getDelay(m.baseValue)
}

func NewFixedDelayModel[T key]() *DelayModel[T, time.Duration] {
	return NewDelayModel[T, time.Duration](func(d time.Duration) time.Duration {
		return d
	})
}

func NewSampledDelayModel[T key, distribution Distribution]() *DelayModel[T, Distribution] {
	return NewDelayModel[T, Distribution](func(d Distribution) time.Duration {
		if d == nil {
			return 0
		}
		return d.SampleDuration()
	})
}
