package broadcast

import (
	"github.com/0xsoniclabs/daphne/daphne/p2p"
)

// Factory is a factory function type for creating new Channel instances.
// It is used to allow dependency injection of different Channel
// implementations in components like the TxPool or Consensus modules.
//
// A call to the factory function should return a new instance of a Channel
// using the provided p2p.Server and may use the provided extractKeyFromMessage
// to determine unique keys for messages of type M.
type Factory[K comparable, M p2p.Message] func(
	p2pServer p2p.Server,
	extractKeyFromMessage func(M) K,
) Channel[M]

// DefaultFactory returns the default [Factory] for the given key type K
// and message type M. Currently, the default factory is [NewGossip]. However,
// this may change in future versions, so users should not rely on this behavior
// if they want to ensure a specific factory is used.
func DefaultFactory[K comparable, M p2p.Message]() Factory[K, M] {
	return NewGossip[K, M]
}

// NewDefault is a convenience function that creates a new Channel instance
// using the default factory for the given key type K and message type M.
// It is a short-cut for
//
//	factory := DefaultFactory[K, M]()
//	channel := factory(p2pServer, extractKeyFromMessage)
func NewDefault[K comparable, M p2p.Message](
	server p2p.Server,
	extractKeyFromMessage func(M) K,
) Channel[M] {
	return DefaultFactory[K, M]()(server, extractKeyFromMessage)
}
