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

package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStore_ZeroStoreIsEmpty(t *testing.T) {
	store := store{}
	require.Len(t, store.data, 0, "Store should be empty")
}

func TestStore_Add_AddsAnElement(t *testing.T) {
	store := store{}
	event1 := &Event{creator: 1}

	store.add(event1)

	_, present := store.get(event1.EventId())
	require.True(t, present)
	require.Len(t, store.data, 1, "Store should contain one element")
}

func TestStore_AddAndGetAreDataRaceFree(t *testing.T) {
	store := store{}
	event1 := &Event{creator: 1}
	event2 := &Event{creator: 2}

	// Add events concurrently
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		store.add(event1)
	}()
	go func() {
		defer wg.Done()
		store.add(event2)
	}()

	// Get events concurrently
	go func() {
		defer wg.Done()
		_, _ = store.get(event1.EventId())
	}()
	go func() {
		defer wg.Done()
		_, _ = store.get(event2.EventId())
	}()

	wg.Wait()
}
