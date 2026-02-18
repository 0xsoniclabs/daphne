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
