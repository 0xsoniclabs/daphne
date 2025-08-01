package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStore_ZeroStoreIsEmpty(t *testing.T) {
	store := &Store{}
	_, present := store.Get(EventId{1})
	require.False(t, present, "Store should not contain any events")
}

func TestStore_Add_AddsAnElementStore(t *testing.T) {
	store := &Store{}
	event1 := Event{Creator: 1}
	event2 := Event{Creator: 2}

	_, present := store.Get(event1.EventId())
	require.False(t, present)

	_, present = store.Get(event2.EventId())
	require.False(t, present)

	store.add(event1)

	_, present = store.Get(event1.EventId())
	require.True(t, present)

	_, present = store.Get(event2.EventId())
	require.False(t, present)
}

func TestStore_AddAndGetAreDataRaceFree(t *testing.T) {
	store := &Store{}
	event1 := Event{Creator: 1}
	event2 := Event{Creator: 2}

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
		_, _ = store.Get(event1.EventId())
	}()
	go func() {
		defer wg.Done()
		_, _ = store.Get(event2.EventId())
	}()

	wg.Wait()
}
