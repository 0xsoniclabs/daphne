// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

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
// optional custom values keyed by any comparable type K.
type DelayModel[K comparable] struct {
	baseValue    Distribution
	customValues map[K]Distribution
	mutex        sync.RWMutex
}

// NewDelayModel constructs a DelayModel parameterized by the key type.
func NewDelayModel[K comparable]() *DelayModel[K] {
	return &DelayModel[K]{customValues: make(map[K]Distribution)}
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
		return custom.SampleDuration()
	}

	if m.baseValue == nil {
		return 0
	}
	return m.baseValue.SampleDuration()
}
