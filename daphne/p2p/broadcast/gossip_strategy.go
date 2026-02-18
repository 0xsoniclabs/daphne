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
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

//go:generate mockgen -source gossip_strategy.go -destination=gossip_strategy_mock.go -package=broadcast

// GossipStrategy defines the interface for controlling when messages should
// be gossiped to peers. Implementations can track message knowledge and
// decide on gossip behavior based on various policies.
type GossipStrategy[K comparable] interface {
	// ShouldGossip checks if a message should be gossiped to a peer.
	ShouldGossip(peer p2p.PeerId, messageKey K) bool
	// OnReceived is called when a message is received from a peer.
	OnReceived(peer p2p.PeerId, messageKey K)
	// OnSent is called when a message is successfully sent to a peer.
	OnSent(peer p2p.PeerId, messageKey K)
	// OnSendFailed is called when sending a message to a peer fails.
	OnSendFailed(peer p2p.PeerId, messageKey K, err error)
}

// === Generic Gossip Strategies ===

// --- Flood Fallback Strategy ---

// FloodFallbackStrategy is a GossipStrategy that gossips a message to a peer
// only if it hasn't explicitly received the message from or sent to that peer
// before. This is a "flood if not sure" approach that maximizes message
// propagation.
type FloodFallbackStrategy[K comparable] struct {
	messagesKnownByPeers      map[p2p.PeerId]*sets.Set[K]
	messagesKnownByPeersMutex sync.Mutex
}

// NewFloodFallbackStrategy creates a new FloodFallbackStrategy instance.
func NewFloodFallbackStrategy[K comparable]() *FloodFallbackStrategy[K] {
	return &FloodFallbackStrategy[K]{
		messagesKnownByPeers: make(map[p2p.PeerId]*sets.Set[K]),
	}
}

func (s *FloodFallbackStrategy[K]) ShouldGossip(
	peer p2p.PeerId,
	messageKey K,
) bool {
	s.messagesKnownByPeersMutex.Lock()
	defer s.messagesKnownByPeersMutex.Unlock()

	if _, exists := s.messagesKnownByPeers[peer]; !exists {
		return true
	}
	if !s.messagesKnownByPeers[peer].Contains(messageKey) {
		return true
	}
	return false
}

func (s *FloodFallbackStrategy[K]) OnReceived(peer p2p.PeerId, messageKey K) {
	s.markMessageKnownByPeer(peer, messageKey)
}

func (s *FloodFallbackStrategy[K]) OnSent(peer p2p.PeerId, messageKey K) {
	s.markMessageKnownByPeer(peer, messageKey)
}

func (s *FloodFallbackStrategy[K]) OnSendFailed(
	peer p2p.PeerId,
	messageKey K,
	err error,
) {
	// Still mark as known even on failure to avoid retry loops
	s.markMessageKnownByPeer(peer, messageKey)
}

func (s *FloodFallbackStrategy[K]) markMessageKnownByPeer(
	peer p2p.PeerId,
	messageKey K,
) {
	s.messagesKnownByPeersMutex.Lock()
	defer s.messagesKnownByPeersMutex.Unlock()

	if _, exists := s.messagesKnownByPeers[peer]; !exists {
		set := sets.Empty[K]()
		s.messagesKnownByPeers[peer] = &set
	}
	s.messagesKnownByPeers[peer].Add(messageKey)
}

// --- No Gossip Strategy ---

// NoGossipStrategy is a GossipStrategy that never gossips messages.
// For testing or for nodes that should only receive but not propagate.
type NoGossipStrategy[K comparable] struct{}

// NewNoGossipStrategy creates a new NoGossipStrategy instance.
func NewNoGossipStrategy[K comparable]() *NoGossipStrategy[K] {
	return &NoGossipStrategy[K]{}
}

func (s *NoGossipStrategy[K]) ShouldGossip(peer p2p.PeerId, messageKey K) bool {
	return false
}

func (s *NoGossipStrategy[K]) OnReceived(peer p2p.PeerId, messageKey K) {
	// No-op
}

func (s *NoGossipStrategy[K]) OnSent(peer p2p.PeerId, messageKey K) {
	// No-op
}

func (s *NoGossipStrategy[K]) OnSendFailed(
	peer p2p.PeerId,
	messageKey K,
	err error,
) {
	// No-op
}
