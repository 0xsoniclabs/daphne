package p2p

import (
	"fmt"
	"sync"
)

// Network is a P2P network that manages the inter-connection of peers and
// forwards messages between those peers.
type Network struct {
	peers map[PeerId]peer

	tasks sync.WaitGroup
}

// NewNetwork creates a new, empty P2P network.
func NewNetwork() *Network {
	return &Network{
		peers: make(map[PeerId]peer),
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
	// This function is the main component of the simulated P2P network. To
	// emulate latency, delays in the forwarding process are going to be added
	// to this function in the future.
	if _, exists := n.peers[from]; !exists {
		return fmt.Errorf("cannot send message from peer %s: not connected", from)
	}
	if _, exists := n.peers[to]; !exists {
		return fmt.Errorf("cannot send message to peer %s: not connected", to)
	}

	n.tasks.Add(1)
	go func() {
		defer n.tasks.Done()
		n.peers[to].receiveMessage(from, msg)
	}()
	return nil
}

// WaitForMessagesBeingDelivered blocks until all messages in the network have
// been delivered.
// This is a helper tool which prevents having to add sleep waits to guarantee
// delivery of asynchronous messages sent using [server.SendMessage]
//
// This function relies on synchronous message processing within each peer protocol
// and it needs to be called from the same goroutine that sends messages. Protocols
// implementing delayed message forwarding are not compatible with this function and
// other synchronization mechanisms shall be used to guarantee test completion.
//
// Protocols with unconstrained forwarding of messages in the network may lead
// to infinite wait time.
func (n *Network) WaitForDeliveryOfSentMessages() {
	n.tasks.Wait()
}
