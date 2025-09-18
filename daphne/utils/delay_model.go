package utils

import (
	"sync"
)

// make a type key that can be either a string or an uint or renames of either
type key interface {
	~string | ~uint64 | ~uint32
}

type DelayModel[T key, K any] interface {
	GetDelay(T, T) K
	SetBaseDelay(K)
	SetCustomDelay(from, to T, delay K)
}

// make a generic connection type that stores two elements of the same type
type connectionKey[T key] struct {
	from T
	to   T
}

type FixedDelayModel[T key, K any] struct {
	baseDelay    K
	customDelays map[connectionKey[T]]K
	// delaysMutex protects access to all delays.
	delaysMutex sync.RWMutex
}

func NewFixedDelayModel[T key, K any]() *FixedDelayModel[T, K] {
	return &FixedDelayModel[T, K]{
		customDelays: make(map[connectionKey[T]]K),
	}
}

func (m *FixedDelayModel[T, K]) SetBaseDelay(delay K) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.baseDelay = delay
}

func (m *FixedDelayModel[T, K]) SetCustomDelay(from, to T, delay K) {
	m.delaysMutex.Lock()
	defer m.delaysMutex.Unlock()
	m.customDelays[connectionKey[T]{from, to}] = delay
}

func (m *FixedDelayModel[T, K]) GetDelay(from, to T) K {
	m.delaysMutex.RLock()
	defer m.delaysMutex.RUnlock()
	if delay, exists := m.customDelays[connectionKey[T]{from, to}]; exists {
		return delay
	}
	return m.baseDelay
}
