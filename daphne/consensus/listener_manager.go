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
	listeners      []BundleListener
	listenersMutex sync.Mutex

	bundles      chan<- types.Bundle
	bundlesMutex sync.Mutex

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
			manager.listenersMutex.Lock()
			listeners := slices.Clone(manager.listeners)
			manager.listenersMutex.Unlock()
			for _, listener := range listeners {
				listener.OnNewBundle(bundle)
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
}

func (m *BundleListenerManager) NotifyListeners(bundle types.Bundle) {
	m.bundlesMutex.Lock()
	defer m.bundlesMutex.Unlock()
	if m.bundles != nil {
		m.bundles <- bundle
	}
}
