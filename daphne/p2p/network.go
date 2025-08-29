package p2p

import (
	"fmt"
	"sync"
	"time"
)

// Network is a P2P network that manages the inter-connection of peers and
// forwards messages between those peers.
type Network struct {
	peers   map[PeerId]peer
	tasks   sync.WaitGroup
	latency LatencyModel
}

// NewNetwork creates a new, empty P2P network.
func NewNetwork() *Network {
	return NewNetworkWithLatency(nil)
}

// NewNetworkWithLatency creates a new P2P network with a custom latency model.
func NewNetworkWithLatency(model LatencyModel) *Network {
	return &Network{
		peers:   make(map[PeerId]peer),
		latency: model,
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
	for otherId, peer := range n.peers {
		peer.connectTo(id)
		res.connectTo(otherId)
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

	var delay time.Duration
	if n.latency != nil {
		delay = n.latency.GetDelay(from, to, msg)
	}

	n.tasks.Add(1)
	go func() {
		defer n.tasks.Done()
		if delay > 0 {
			time.Sleep(delay)
		}
		n.peers[to].receiveMessage(from, msg)
	}()
	return nil
}

// WaitForMessagesBeingDelivered blocks until all messages in the network have
// been delivered.
// This is a helper tool which prevents having to add sleep waits to guarantee
// delivery of asynchronous messages sent using [server.SendMessage]
//
// This function relies on synchronous message processing within each peer
// protocol and it needs to be called from the same goroutine that sends
// messages. Protocols that implement delayed message forwarding on top of
// asynchronous message processing at the network level, which in turn use
// their own logic to buffer and dispatch messages, are not compatible with this
// function. Other synchronizaton mechanisms would be required on top.
//
// Protocols with unconstrained forwarding of messages in the network may lead
// to infinite wait time.
func (n *Network) WaitForDeliveryOfSentMessages() {
	n.tasks.Wait()
}
