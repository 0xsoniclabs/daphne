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
	id       PeerId
	peers    []PeerId
	handlers []MessageHandler
	network  *Network
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

func (s *server) receiveMessage(from PeerId, msg Message) {
	for _, handler := range s.handlers {
		handler.HandleMessage(from, msg)
	}
}

func (s *server) connectTo(peerId PeerId) {
	if !slices.Contains(s.peers, peerId) {
		s.peers = append(s.peers, peerId)
	}
}
