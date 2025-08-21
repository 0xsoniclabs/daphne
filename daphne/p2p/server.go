package p2p

import (
	"fmt"
	"slices"
)

//go:generate mockgen -source server.go -destination=server_mock.go -package=p2p

// Server is a P2P server run by each node on a network. It is the node-facing
// interface that can be used by other components to interact with the P2P
// network.
type Server interface {
	// GetLocalId returns the unique identifier of this node in the P2P network.
	GetLocalId() PeerId
	// GetPeers returns a list of peers this server is directly connected to.
	GetPeers() []PeerId
	// SendMessage sends the given message to the specified peer.
	SendMessage(to PeerId, msg Message) error
	// ReceiveMessage is called by the network to deliver a message from a peer.
	ReceiveMessage(from PeerId, msg Message)

	// RegisterMessageHandler registers a new handler for incoming messages.
	// Every future message received by this node will be passed to this handler.
	RegisterMessageHandler(handler MessageHandler)
}

// MessageHandler is an interface for handling messages received from peers.
type MessageHandler interface {
	HandleMessage(from PeerId, msg Message)
}

// --- Server implementation ---

type server struct {
	id    PeerId
	peers []PeerId // TODO: check if peers is redundant
	// there are no functions to constrain connections
	handlers []MessageHandler
	network  *Network
}

func (n *Network) NewServer(id PeerId) (Server, error) {
	// Check that the new server is not reusing an existing ID.
	if _, exists := n.servers[id]; exists {
		return nil, fmt.Errorf("server with ID %s already exists", id)
	}

	s := &server{
		id:       id,
		handlers: []MessageHandler{},
		network:  n,
	}

	for otherId, serverInstance := range n.servers {
		if serverImpl, ok := serverInstance.(*server); ok {
			serverImpl.connectTo(id)
			s.connectTo(otherId)
		}
	}

	n.servers[id] = s
	n.channels[id] = make(chan asyncMessage, 1000) // Create a channel for this server

	go func() {
		for {
			select {
			case asyncMsg := <-n.channels[id]:
				s.ReceiveMessage(asyncMsg.from, asyncMsg.msg)
			case <-n.shutdown:
				return // Exit goroutine on shutdown
			}
		}
	}()

	return s, nil
}

func (s *server) GetLocalId() PeerId {
	return s.id
}

func (s *server) GetPeers() []PeerId {
	return s.peers
}

func (s *server) SendMessage(to PeerId, msg Message) error {
	if !slices.Contains(s.peers, to) {
		return fmt.Errorf("cannot send message to peer %s: not connected", to)
	}
	return s.network.transferMessage(s.id, to, msg)
}

func (s *server) RegisterMessageHandler(handler MessageHandler) {
	s.handlers = append(s.handlers, handler)
}

func (s *server) ReceiveMessage(from PeerId, msg Message) {
	for _, handler := range s.handlers {
		handler.HandleMessage(from, msg)
	}
}

func (s *server) connectTo(peerId PeerId) {
	if !slices.Contains(s.peers, peerId) {
		s.peers = append(s.peers, peerId)
	}
}
