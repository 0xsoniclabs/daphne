package generic

import (
	"slices"
	"sync"
)

//go:generate mockgen -source broadcast.go -destination=broadcast_mock.go -package=generic

// Broadcaster is a policy for broadcasting messages of type M to multiple receivers.
// No assumptions are made about the distribution policy - they could be best-effort
// only or a guaranteed delivery protocol, with or without timing guarantees.
type Broadcaster[M any] interface {
	Broadcast(message M)
	RegisterReceiver(receiver BroadcastReceiver[M])
	UnregisterReceiver(receiver BroadcastReceiver[M])
}

// BroadcastReceiver is an interface for receiving messages of type M from a
// broadcaster protocol. Callbacks are guaranteed to be called asynchronously
// to the `Broadcast` calls, but no assumptions are made about the timing or
// ordering of the messages.
type BroadcastReceiver[M any] interface {
	OnMessage(message M)
}

// --- Adapter for functions to BroadcastReceiver ---

// WrapBroadcastReceiver wraps a function with the signature
// func(message M) into a BroadcastReceiver adapter that can be registered
// on a [Broadcaster]. This is a convenience function to allow using simple
// functions as message handlers without having to define a new type.
func WrapBroadcastReceiver[M any](f func(message M)) BroadcastReceiver[M] {
	return &broadcastReceiverAdapter[M]{f: f}
}

type broadcastReceiverAdapter[M any] struct {
	f func(message M)
}

func (h *broadcastReceiverAdapter[M]) OnMessage(message M) {
	h.f(message)
}

// --- BroadcastReceivers registry ---

// BroadcastReceivers is a thread-safe registry of BroadcastReceiver instances.
// It allows registering, unregistering, and delivering messages to all
// registered receivers and is intended as a building block for broadcaster
// implementations.
type BroadcastReceivers[M any] struct {
	receivers []BroadcastReceiver[M]
	mutex     sync.Mutex
}

// Register adds a receiver to the registry. If the receiver is already
// registered, it is not added again.
// This operation is thread safe.
func (g *BroadcastReceivers[M]) Register(receiver BroadcastReceiver[M]) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	contained := slices.ContainsFunc(g.receivers, func(r BroadcastReceiver[M]) bool {
		return r == receiver
	})
	if !contained {
		g.receivers = append(g.receivers, receiver)
	}
}

// Unregister removes a receiver from the registry. If the receiver is not
// registered, this is a no-op.
// This operation is thread safe.
func (g *BroadcastReceivers[M]) Unregister(receiver BroadcastReceiver[M]) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.receivers = slices.DeleteFunc(g.receivers, func(r BroadcastReceiver[M]) bool {
		return r == receiver
	})
}

// Deliver sends the given message to all registered receivers asynchronously.
// The delivery is thread safe. However, there are no guarantees about the
// timing or ordering of the messages. Also, if receivers are added or removed
// during delivery, it is not guaranteed whether they will receive the message
// or not.
// This operation is thread safe. Calls to OnMessage are done asynchronously.
// The function does not wait for the calls to complete.
func (g *BroadcastReceivers[M]) Deliver(message M) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	for _, receiver := range g.receivers {
		go receiver.OnMessage(message)
	}
}
