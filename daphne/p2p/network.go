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

package p2p

import (
	"fmt"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
)

// Network is a P2P network that manages the inter-connection of peers and
// forwards messages between those peers.
type Network struct {
	peers      map[PeerId]peer
	peersLock  sync.Mutex
	latency    LatencyModel
	topology   NetworkTopology
	tracker    tracker.Tracker
	msgCounter atomic.Uint32
}

// NewNetwork creates a new, empty P2P network.
func NewNetwork() *Network {
	return NewNetworkBuilder().Build()
}

// NetworkBuilder is a builder for creating a new P2P network with custom
// configurations.
type NetworkBuilder struct {
	latency  LatencyModel
	topology NetworkTopology
	tracker  tracker.Tracker
}

// NewNetworkBuilder creates a new instance of NetworkBuilder creating a network
// with its default configuration.
func NewNetworkBuilder() *NetworkBuilder {
	return &NetworkBuilder{}
}

// WithLatency sets the latency model to be used by the network being built.
func (b *NetworkBuilder) WithLatency(latency LatencyModel) *NetworkBuilder {
	b.latency = latency
	return b
}

// WithTopology sets the topology model to be used by the network being built.
func (b *NetworkBuilder) WithTopology(topology NetworkTopology) *NetworkBuilder {
	b.topology = topology
	return b
}

// WithTracker sets the event tracker to be used by the network being built.
func (b *NetworkBuilder) WithTracker(tracker tracker.Tracker) *NetworkBuilder {
	b.tracker = tracker
	return b
}

// Build constructs the Network instance from the builder's configuration.
func (b *NetworkBuilder) Build() *Network {
	// Default to a fully-meshed network where peers automatically connect as
	// they are added.
	topology := b.topology
	if topology == nil {
		topology = NewFullyMeshedTopology()
	}
	return &Network{
		latency:  b.latency,
		topology: topology,
		tracker:  b.tracker,
		peers:    make(map[PeerId]peer),
	}
}

// NewServer creates a new server on this P2P network with the given PeerId. The
// resulting server instance can be used by a node to interact with the network.
func (n *Network) NewServer(id PeerId) (Server, error) {
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	// Check that the new server is not reusing an existing ID.
	if _, exists := n.peers[id]; exists {
		return nil, fmt.Errorf("server with ID %s already exists", id)
	}
	res := newServer(id, n)

	// For each existing peer, check for connections in both directions based on
	// the network's current topology.
	for otherId, otherPeer := range n.peers {
		// Check if the new peer should connect to the existing peer.
		if n.topology.ShouldConnect(id, otherId) {
			res.connectTo(otherId)
		}
		// Check if the existing peer should connect to the new peer.
		if n.topology.ShouldConnect(otherId, id) {
			otherPeer.connectTo(id)
		}
	}

	n.peers[id] = res
	return res, nil
}

// UpdateTopology resets all peer connections and re-establishes them based on
// the provided network topology. This is useful for topologies that require
// the complete set of peers to be known in advance before connections are made.
func (n *Network) UpdateTopology(topology NetworkTopology) {
	n.peersLock.Lock()
	defer n.peersLock.Unlock()
	n.topology = topology

	// Clear all existing connections for every peer.
	for _, p := range n.peers {
		p.clearConnections()
	}

	allPeerIds := slices.Collect(maps.Keys(n.peers))

	// Re-establish connections based on the new topology by checking every
	// possible directed connection between pairs of peers.
	for _, fromId := range allPeerIds {
		peerFrom := n.peers[fromId]
		for _, toId := range allPeerIds {
			if fromId == toId {
				continue
			}

			// If the topology dictates a connection should exist, create it.
			if n.topology.ShouldConnect(fromId, toId) {
				peerFrom.connectTo(toId)
			}
		}
	}
}

// transferMessage sends a message from one peer to another within the network.
// The transfer will fail if either the sender or receiver is not connected to
// the network.
func (n *Network) transferMessage(from PeerId, to PeerId, msg Message) error {
	n.peersLock.Lock()
	if _, exists := n.peers[from]; !exists {
		n.peersLock.Unlock()
		return fmt.Errorf("cannot send message from peer %s: not connected", from)
	}
	if _, exists := n.peers[to]; !exists {
		n.peersLock.Unlock()
		return fmt.Errorf("cannot send message to peer %s: not connected", to)
	}
	receiver := n.peers[to]
	n.peersLock.Unlock()

	id := n.msgCounter.Add(1)
	if n.latency != nil {
		time.Sleep(n.latency.GetSendDelay(from, to, msg))
	}
	if n.tracker != nil {
		n.tracker.Track(mark.MsgSent, "id", id, "from", from, "to", to,
			"type", GetMessageType(msg), "bytesize", GetMessageSize(msg))
	}

	go func() {
		if n.latency != nil {
			time.Sleep(n.latency.GetDeliveryDelay(from, to, msg))
		}
		if n.tracker != nil {
			n.tracker.Track(mark.MsgReceived, "id", id, "from", from, "to", to,
				"type", GetMessageType(msg), "bytesize", GetMessageSize(msg))
		}
		receiver.receiveMessage(from, msg)
		if n.tracker != nil {
			n.tracker.Track(mark.MsgConsumed, "id", id, "from", from, "to", to,
				"type", GetMessageType(msg), "bytesize", GetMessageSize(msg))
		}
	}()
	return nil
}
