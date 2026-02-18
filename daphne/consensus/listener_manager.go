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

package consensus

import (
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

// BundleListenerManager manages a set of BundleListeners for a consensus
// protocol implementation, ensuring that bundle announcements are propagated
// to all registered listeners asynchronously, but in order.
type BundleListenerManager struct {
	listeners             []BundleListener
	nextBundleForListener []int
	listenersMutex        sync.Mutex

	bundles      chan<- types.Bundle
	bundlesMutex sync.Mutex

	bundleHistory []types.Bundle

	done <-chan struct{}
}

func NewBundleListenerManager() *BundleListenerManager {
	bundles := make(chan types.Bundle, 100)
	done := make(chan struct{})
	manager := &BundleListenerManager{
		bundles: bundles,
		done:    done,
	}
	go func() {
		defer close(done)
		for bundle := range bundles {
			manager.bundleHistory = append(manager.bundleHistory, bundle)
			manager.listenersMutex.Lock()
			listeners := slices.Clone(manager.listeners)
			nextBundle := slices.Clone(manager.nextBundleForListener)
			manager.listenersMutex.Unlock()
			for i, listener := range listeners {
				for j := nextBundle[i]; j < len(manager.bundleHistory); j++ {
					listener.OnNewBundle(manager.bundleHistory[j])
					nextBundle[i]++
				}
				manager.listenersMutex.Lock()
				manager.nextBundleForListener[i] = nextBundle[i]
				manager.listenersMutex.Unlock()
			}
		}
	}()
	return manager
}

func (m *BundleListenerManager) Stop() {
	m.bundlesMutex.Lock()
	defer m.bundlesMutex.Unlock()
	if m.bundles == nil {
		return
	}
	close(m.bundles)
	<-m.done
	m.bundles = nil
}

func (m *BundleListenerManager) RegisterListener(listener BundleListener) {
	if listener == nil {
		return
	}
	m.listenersMutex.Lock()
	defer m.listenersMutex.Unlock()
	m.listeners = append(m.listeners, listener)
	m.nextBundleForListener = append(m.nextBundleForListener, 0)
}

func (m *BundleListenerManager) NotifyListeners(bundle types.Bundle) {
	m.bundlesMutex.Lock()
	defer m.bundlesMutex.Unlock()
	if m.bundles != nil {
		m.bundles <- bundle
	}
}
