package p2p

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
)

// Network is a P2P network that manages the inter-connection of peers and
// forwards messages between those peers.
type Network struct {
	peers      map[PeerId]peer
	latency    LatencyModel
	topology   TopologyModel
	tracker    tracker.Tracker
	msgCounter atomic.Uint64
}

// NewNetwork creates a new, empty P2P network.
func NewNetwork() *Network {
	return NewNetworkBuilder().Build()
}

// NetworkBuilder is a builder for creating a new P2P network with custom
// configurations.
type NetworkBuilder struct {
	latency  LatencyModel
	topology TopologyModel
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
func (b *NetworkBuilder) WithTopology(topology TopologyModel) *NetworkBuilder {
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
	// Ensure a default topology if none is provided.
	topology := b.topology
	if topology == nil {
		topology = NewFullyConnectedTopology()
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
	// Check that the new server is not reusing an existing ID.
	if _, exists := n.peers[id]; exists {
		return nil, fmt.Errorf("server with ID %s already exists", id)
	}
	res := &server{
		id:       id,
		peers:    []PeerId{},
		handlers: []MessageHandler{},
		network:  n,
	}

	// For each existing peer, check for connections in both directions.
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

// transferMessage sends a message from one peer to another within the network.
// The transfer will fail if either the sender or receiver is not connected to
// the network.
func (n *Network) transferMessage(from PeerId, to PeerId, msg Message) error {
	if _, exists := n.peers[from]; !exists {
		return fmt.Errorf("cannot send message from peer %s: not connected", from)
	}
	if _, exists := n.peers[to]; !exists {
		return fmt.Errorf("cannot send message to peer %s: not connected", to)
	}

	id := n.msgCounter.Add(1)
	if n.latency != nil {
		time.Sleep(n.latency.GetSendDelay(from, to, msg))
	}
	if n.tracker != nil {
		n.tracker.Track(mark.MsgSent, "id", id, "from", from, "to", to,
			"type", msg.Code)
	}

	go func() {
		if n.latency != nil {
			time.Sleep(n.latency.GetDeliveryDelay(from, to, msg))
		}
		if n.tracker != nil {
			n.tracker.Track(mark.MsgReceived, "id", id, "from", from, "to", to,
				"type", msg.Code)
		}
		n.peers[to].receiveMessage(from, msg)
		if n.tracker != nil {
			n.tracker.Track(mark.MsgConsumed, "id", id, "from", from, "to", to,
				"type", msg.Code)
		}
	}()
	return nil
}
