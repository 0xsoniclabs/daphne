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
	if m == nil {
		return
	}
	m.bundlesMutex.Lock()
	defer m.bundlesMutex.Unlock()
	if m.bundles != nil {
		m.bundles <- bundle
	}
}
