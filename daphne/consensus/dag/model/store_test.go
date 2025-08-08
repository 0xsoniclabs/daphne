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
