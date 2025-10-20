package broadcast

import (
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
)

//go:generate mockgen -source channel.go -destination=channel_mock.go -package=broadcast

// Channel is a communication channel for broadcasting messages of type M to
// multiple receivers. No assumptions are made about the distribution policy.
// They could be best-effort only or a guaranteed delivery protocol, with or
// without timing guarantees.
type Channel[M p2p.Message] interface {
	Broadcast(message M)
	Register(receiver Receiver[M])
	Unregister(receiver Receiver[M])
}

// Receiver is an interface for receiving messages of type M from a channel.
// Callbacks are guaranteed to be called asynchronously to the `Broadcast` calls
// on the sender side, but no assumptions are made about the timing or
// ordering of the messages.
type Receiver[M p2p.Message] interface {
	OnMessage(message M)
}

// --- Adapter for functions to Receiver ---

// WrapReceiver wraps a function with the signature func(message M) into a
// Receiver adapter that can be registered on a [Channel]. This is a convenience
// function to allow using simple functions as message handlers without having
// to define a new type.
func WrapReceiver[M p2p.Message](f func(message M)) Receiver[M] {
	return &receiverAdapter[M]{f: f}
}

type receiverAdapter[M p2p.Message] struct {
	f func(message M)
}

func (h *receiverAdapter[M]) OnMessage(message M) {
	h.f(message)
}

// --- Receivers registry ---

// Receivers is a thread-safe registry of Receiver instances.
// It allows registering, unregistering, and delivering messages to all
// registered receivers and is intended as a building block for channel
// implementations.
type Receivers[M p2p.Message] struct {
	receivers []Receiver[M]
	mutex     sync.Mutex
}

// Register adds a receiver to the registry. If the receiver is already
// registered, it is not added again.
// This operation is thread safe.
func (g *Receivers[M]) Register(receiver Receiver[M]) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	contained := slices.ContainsFunc(g.receivers, func(r Receiver[M]) bool {
		return r == receiver
	})
	if !contained {
		g.receivers = append(g.receivers, receiver)
	}
}

// Unregister removes a receiver from the registry. If the receiver is not
// registered, this is a no-op.
// This operation is thread safe.
func (g *Receivers[M]) Unregister(receiver Receiver[M]) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.receivers = slices.DeleteFunc(g.receivers, func(r Receiver[M]) bool {
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
func (g *Receivers[M]) Deliver(message M) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	for _, receiver := range g.receivers {
		go receiver.OnMessage(message)
	}
}
