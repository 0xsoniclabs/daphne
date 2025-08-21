package p2p

import (
	"fmt"
)

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
	servers    map[PeerId]Server            // All servers in the network
	peers      map[PeerId]peer              // All connected peers in the network
	channels   map[PeerId]chan asyncMessage // Channels for direct peer communication
	shutdown   chan struct{}                // Shutdown signal
	workerDone chan struct{}                // Worker completion signal
}

// NewNetwork creates a new, empty P2P network with asynchronous message
// delivery. The asyncConfig parameter configures the behavior of message
// delivery. If nil, default async behavior is used.
func NewNetwork() *Network {
	n := &Network{
		servers:    make(map[PeerId]Server),
		peers:      make(map[PeerId]peer),
		channels:   make(map[PeerId]chan asyncMessage),
		shutdown:   make(chan struct{}),
		workerDone: make(chan struct{}),
	}
	return n
}

func (n *Network) ConnectServer(server Server) error {
	id := server.GetLocalId()
	// Check that the new server is not reusing an existing ID.
	if _, exists := n.peers[id]; exists {
		return fmt.Errorf("server with ID %s already exists", id)
	}

	n.servers[id] = server
	n.channels[id] = make(chan asyncMessage, 1000) // Create a channel for this server

	go func() {
		for {
			select {
			case asyncMsg := <-n.channels[id]:
				// Deliver messages to the server's message handlers
				server.ReceiveMessage(asyncMsg.from, asyncMsg.msg)
			case <-n.shutdown:
				return // Exit goroutine on shutdown
			}
		}
	}()

	return nil
}

// transferMessage sends a message from one peer to another within the network.
// The transfer will fail if either the sender or receiver is not connected to
// the network. All messages are queued for asynchronous delivery by a
// background worker.
func (n *Network) transferMessage(from PeerId, to PeerId, msg Message) error {
	if _, exists := n.servers[from]; !exists {
		return fmt.Errorf("cannot send message from peer %s: not connected", from)
	}
	if _, exists := n.servers[to]; !exists {
		return fmt.Errorf("cannot send message to peer %s: not connected", to)
	}

	asyncMessage := asyncMessage{
		from: from,
		to:   to,
		msg:  msg,
	}

	// Asynchronous delivery via worker goroutine
	select {
	case n.channels[to] <- asyncMessage:
		return nil
	default:
		return fmt.Errorf("message queue full (capacity: 1000)")
	}
}

// // startWorker starts a single background goroutine that processes messages
// // from the message queue for asynchronous delivery.
// func (n *Network) startWorker() {
// 	go func() {
// 		defer close(n.workerDone)
// 		for {
// 			select {
// 			case asyncMsg := <-n.messageQueue:
// 				// Handle each message asynchronously to avoid blocking the worker
// 				go n.processMessage(asyncMsg)
// 			case <-n.shutdown:
// 				// Drain remaining messages before shutdown
// 				n.drainQueue()
// 				return
// 			}
// 		}
// 	}()
// }

// drainQueue processes all messages in the queue until it is empty. This is a
// best effort/behaviour on shutdown, since we should note that there are
// zero trust assumptions in our network model. This means that peers are not
// responsible for processing everything in their queue as they are shutdown.
// func (n *Network) drainQueue() {
// 	for {
// 		select {
// 		case asyncMsg := <-n.messageQueue:
// 			if peer, exists := n.peers[asyncMsg.to]; exists {
// 				peer.receiveMessage(asyncMsg.from, asyncMsg.msg)
// 			}
// 		default:
// 			return // Queue is empty
// 		}
// 	}
// }

// Shutdown gracefully stops the asynchronous worker.
// This method blocks until all queued messages are processed.
func (n *Network) Shutdown() error {
	close(n.shutdown)
	<-n.workerDone // Wait for worker to finish processing all messages
	return nil
}
