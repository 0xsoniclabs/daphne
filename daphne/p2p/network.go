package p2p

import (
	"fmt"
	"math/rand"
	"time"
)

// AsyncConfig configures asynchronous message delivery behavior.
type AsyncConfig struct {
	// Optional message behavior configurations.
	MessageDelay *time.Duration // Random delay up to this duration
	DropRate     *float64       // Probability of dropping messages (0.0-1.0)
}

// asyncMessage represents an internal message being transferred between peers
// asynchronously.
type asyncMessage struct {
	from PeerId
	to   PeerId
	msg  Message
}

// Network is a P2P network that manages the inter-connection of peers and
// forwards messages between those peers.
type Network struct {
	peers        map[PeerId]peer
	asyncConfig  *AsyncConfig      // Configuration for async behavior
	messageQueue chan asyncMessage // Buffer for async message delivery
	shutdown     chan struct{}     // Shutdown signal
	workerDone   chan struct{}     // Worker completion signal
}

// NewNetwork creates a new, empty P2P network with asynchronous message
// delivery. The asyncConfig parameter configures the behavior of message
// delivery. If nil, default async behavior is used.
func NewNetwork(asyncConfig *AsyncConfig) *Network {
	if asyncConfig == nil {
		asyncConfig = &AsyncConfig{}
	}
	n := &Network{
		peers:        make(map[PeerId]peer),
		asyncConfig:  asyncConfig,
		messageQueue: make(chan asyncMessage, 1000), // Hardcoded buffer size
		shutdown:     make(chan struct{}),
		workerDone:   make(chan struct{}),
	}
	n.startWorker()
	return n
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
// the network. All messages are queued for asynchronous delivery by a
// background worker.
func (n *Network) transferMessage(from PeerId, to PeerId, msg Message) error {
	if _, exists := n.peers[from]; !exists {
		return fmt.Errorf("cannot send message from peer %s: not connected", from)
	}
	if _, exists := n.peers[to]; !exists {
		return fmt.Errorf("cannot send message to peer %s: not connected", to)
	}

	// Asynchronous delivery via worker goroutine
	select {
	case n.messageQueue <- asyncMessage{from, to, msg}:
		return nil
	default:
		return fmt.Errorf("message queue full (capacity: 1000)")
	}
}

// startWorker starts a single background goroutine that processes messages
// from the message queue for asynchronous delivery.
func (n *Network) startWorker() {
	go func() {
		defer close(n.workerDone)
		for {
			select {
			case asyncMsg := <-n.messageQueue:
				// Handle each message asynchronously to avoid blocking the worker
				go n.processMessage(asyncMsg)
			case <-n.shutdown:
				// Drain remaining messages before shutdown
				n.drainQueue()
				return
			}
		}
	}()
}

// processMessage handles individual message processing with configured
// behaviors.
func (n *Network) processMessage(asyncMsg asyncMessage) {
	// Apply drop behavior
	if n.shouldDropMessage() {
		return // Drop message
	}

	// Apply delay (if configured)
	n.delayMessage()

	// Deliver message
	if peer, exists := n.peers[asyncMsg.to]; exists {
		peer.receiveMessage(asyncMsg.from, asyncMsg.msg)
	}
}

// shouldDropMessage determines if a message should be dropped based on
// configuration.
func (n *Network) shouldDropMessage() bool {
	if n.asyncConfig.DropRate != nil && *n.asyncConfig.DropRate > 0 {
		return rand.Float64() < *n.asyncConfig.DropRate
	}
	return false
}

// delayMessage applies a random delay up to the configured maximum delay.
// If no delay is configured, it defaults to zero (no delay).
func (n *Network) delayMessage() {
	if n.asyncConfig.MessageDelay != nil && *n.asyncConfig.MessageDelay > 0 {
		multiplier := rand.Float64() // Random delay multiplier between 0.0 and 1.0
		delay := time.Duration(float64(*n.asyncConfig.MessageDelay) * multiplier)
		time.Sleep(delay)
	}
}

// drainQueue processes all messages in the queue until it is empty. This is a
// best effort/behaviour on shutdown, since we should note that there are
// zero trust assumptions in our network model. This means that peers are not
// responsible for processing everything in their queue as they are shutdown.
func (n *Network) drainQueue() {
	for {
		select {
		case asyncMsg := <-n.messageQueue:
			if peer, exists := n.peers[asyncMsg.to]; exists {
				peer.receiveMessage(asyncMsg.from, asyncMsg.msg)
			}
		default:
			return // Queue is empty
		}
	}
}

// Shutdown gracefully stops the asynchronous worker.
// This method blocks until all queued messages are processed.
func (n *Network) Shutdown() error {
	close(n.shutdown)
	<-n.workerDone // Wait for worker to finish processing all messages
	return nil
}
