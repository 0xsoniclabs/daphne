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

package broadcast

import "github.com/0xsoniclabs/daphne/daphne/p2p"

//go:generate stringer -type=Protocol -output protocols_string.go -trimprefix Protocol

// Protocol is an enumeration of the available broadcast protocols. This
// enumeration is intended to be used in configuration and factory functions to
// select the desired broadcast protocol.
type Protocol int

const (
	// ProtocolGossip indicates the Gossip broadcast protocol.
	ProtocolGossip Protocol = iota
	// ProtocolForwarding indicates the Forwarding broadcast protocol.
	ProtocolForwarding
)

// NewChannel creates a new Channel instance for the given key type K and
// message type M using the provided protocol. This is a short-cut for
//
//	factory := GetFactory[K, M](protocol)
//	channel := factory(p2pServer, extractKeyFromMessage)
func NewChannel[K comparable, M p2p.Message](
	protocol Protocol,
	server p2p.Server,
	extractKeyFromMessage func(M) K,
) Channel[M] {
	factory := GetFactory[K, M](protocol)
	return factory(server, extractKeyFromMessage)
}

// GetFactory returns the [Factory] for the given key type K and message type M
// based on the specified broadcast [Protocol]. It panics if an unknown protocol
// is provided.
func GetFactory[K comparable, M p2p.Message](
	protocol Protocol,
) Factory[K, M] {
	switch protocol {
	case ProtocolGossip:
		return NewGossip[K, M]
	case ProtocolForwarding:
		return NewForwarding[K, M]
	default:
		panic("unknown broadcast factory")
	}
}
