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

package consensus

import (
	"testing"
	"testing/synctest"

	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestBundleListenerManager_DoesNotRegisterNilListener(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := NewBundleListenerManager()
		defer manager.Stop()

		manager.RegisterListener(nil)
		require.Empty(t, manager.listeners)
	})
}

func TestBundleListenerManager_RegisterListener_AddsListener(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := NewBundleListenerManager()
		defer manager.Stop()

		ctrl := gomock.NewController(t)
		listener := NewMockBundleListener(ctrl)

		manager.RegisterListener(listener)
		require.Len(t, manager.listeners, 1)
		require.Equal(t, listener, manager.listeners[0])
	})
}

func TestBundleListenerManager_Stop_RepeatedCallsAreIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := NewBundleListenerManager()

		manager.Stop()
		require.Nil(t, manager.bundles)

		// Calling Stop again should be a no-op
		manager.Stop()
		require.Nil(t, manager.bundles)
	})
}

func TestBundleListenerManager_NotifyListeners_CallsOnNewBundleForAllListeners(t *testing.T) {
	manager := NewBundleListenerManager()
	defer manager.Stop()

	ctrl := gomock.NewController(t)
	listener1 := NewMockBundleListener(ctrl)
	listener2 := NewMockBundleListener(ctrl)

	bundle := types.Bundle{}

	listener1.EXPECT().OnNewBundle(bundle).Times(1)
	listener2.EXPECT().OnNewBundle(bundle).Times(1)

	manager.RegisterListener(listener1)
	manager.RegisterListener(listener2)

	manager.NotifyListeners(bundle)
}

func TestBundleListenerManager_NotifyListeners_DeliversBundlesInOrder(t *testing.T) {
	manager := NewBundleListenerManager()
	defer manager.Stop()

	ctrl := gomock.NewController(t)
	listener := NewMockBundleListener(ctrl)

	bundle1 := types.Bundle{Number: 1}
	bundle2 := types.Bundle{Number: 2}
	bundle3 := types.Bundle{Number: 3}

	gomock.InOrder(
		listener.EXPECT().OnNewBundle(bundle1),
		listener.EXPECT().OnNewBundle(bundle2),
		listener.EXPECT().OnNewBundle(bundle3),
	)

	manager.RegisterListener(listener)

	manager.NotifyListeners(bundle1)
	manager.NotifyListeners(bundle2)
	manager.NotifyListeners(bundle3)
}

func TestBundleListenerManager_NotifyListeners_DeliversBundlesRetroactively(t *testing.T) {
	manager := NewBundleListenerManager()
	defer manager.Stop()

	ctrl := gomock.NewController(t)
	listener := NewMockBundleListener(ctrl)

	bundle1 := types.Bundle{Number: 1}
	bundle2 := types.Bundle{Number: 2}
	bundle3 := types.Bundle{Number: 3}

	gomock.InOrder(
		listener.EXPECT().OnNewBundle(bundle1),
		listener.EXPECT().OnNewBundle(bundle2),
		listener.EXPECT().OnNewBundle(bundle3),
	)

	manager.NotifyListeners(bundle1)
	manager.NotifyListeners(bundle2)

	manager.RegisterListener(listener)

	manager.NotifyListeners(bundle3)
}
