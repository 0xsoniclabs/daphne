package broadcast

import (
	reflect "reflect"

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

// Factories maps each combination of key type K and message type M to a
// corresponding [Factory] function. It is used to register and retrieve
// different [Channel] factory implementations based on the types of keys and
// messages being used.
//
// If no factory is registered for a specific combination of K and M, a default
// factory obtained by [DefaultFactory] is used. To override this default, the
// function [SetFactory] can be used to register a custom factory for a specific
// combination of K and M. Unfortunately, due to Go's type system the generic
// function [SetFactory] cannot be defined as a method on this struct or a
// builder type. The recommended way to configure a Factories instance is thus:
//
//	var factories *Factories  // < uses default for everything
//	factories = SetFactory(factories, NewFlooding[int, int])    // < override for K=int, M=int
//	factories = SetFactory(factories, NewFlooding[string, int]) // < override for K=string, M=int
//
// To obtain a specific factory from a Factories instance, use the
// [GetFactory] function. To create a channel directly, use the [NewChannel]
// function.
type Factories struct {
	// overrides maps key types to message types to factory functions replacing
	// the default factory.
	overrides map[reflect.Type]map[reflect.Type]any
}

// SetFactory replaces the factory for the given key type K and message type M
// in the provided Factories instance. The modified Factories instance is
// returned.
//
// This function allows overriding the default factory for specific type
// combinations. See the [Factories] documentation for usage details.
func SetFactory[K comparable, M p2p.Message](
	factories Factories,
	factory Factory[K, M],
) Factories {
	key := reflect.TypeOf(*new(K))
	msg := reflect.TypeOf(*new(M))
	if factories.overrides == nil {
		factories.overrides = make(map[reflect.Type]map[reflect.Type]any)
	}
	if _, exists := factories.overrides[key]; !exists {
		factories.overrides[key] = make(map[reflect.Type]any)
	}
	factories.overrides[key][msg] = factory
	return factories
}

// GetFactory retrieves the factory for the given key type K and message type M
// from the provided Factories instance. If no factory is registered for the
// specific combination of K and M, the default factory obtained by
// [DefaultFactory] is returned.
//
// This function is used to obtain the appropriate factory for creating
// Channel instances based on the types being used. See the [Factories]
// documentation for usage details.
func GetFactory[K comparable, M p2p.Message](
	factories Factories,
) Factory[K, M] {
	key := reflect.TypeOf(*new(K))
	msg := reflect.TypeOf(*new(M))
	factory := factories.overrides[key][msg]
	if factory == nil {
		return DefaultFactory[K, M]()
	}
	return any(factory).(Factory[K, M])
}

// NewChannel creates a new Channel instance for the given key type K and
// message type M using the provided Factories instance to obtain the appropriate
// factory function. This is a short-cut for
//
//	factory := GetFactory[K, M](factories)
//	channel := factory(p2pServer, extractKeyFromMessage)
func NewChannel[K comparable, M p2p.Message](
	factories Factories,
	server p2p.Server,
	extractKeyFromMessage func(M) K,
) Channel[M] {
	factory := GetFactory[K, M](factories)
	return factory(server, extractKeyFromMessage)
}

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
