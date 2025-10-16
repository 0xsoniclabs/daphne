package p2p

import (
	"fmt"
	"slices"
	"sync"
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
// HandleMessage calls are guaranteed to be called asynchronously to other
// calls, but no guarantees are made about the timing or ordering of incoming
// messages.
type MessageHandler interface {
	HandleMessage(from PeerId, msg Message)
}

// --- Adapter for functions to MessageHandler ---

// WrapMessageHandler wraps a function with the signature
// func(from PeerId, msg Message) into a MessageHandler that can be registered
// with a Server. This is a convenience function to allow using simple
// functions as message handlers without having to define a new type.
func WrapMessageHandler(f func(from PeerId, msg Message)) MessageHandler {
	return lambdaMessageHandler(f)
}

type lambdaMessageHandler func(from PeerId, msg Message)

func (h lambdaMessageHandler) HandleMessage(from PeerId, msg Message) {
	h(from, msg)
}

// --- Server implementation ---

type server struct {
	id          PeerId
	peers       []PeerId
	peerLock    sync.Mutex
	handlers    []MessageHandler
	handlerLock sync.Mutex
	network     *Network
}

func newServer(id PeerId, network *Network) *server {
	return &server{
		id:       id,
		peers:    []PeerId{},
		handlers: []MessageHandler{},
		network:  network,
	}
}

func (s *server) GetLocalId() PeerId {
	return s.id
}

func (s *server) GetPeers() []PeerId {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()
	// Return a copy to prevent race conditions on the caller's side.
	peersCopy := make([]PeerId, len(s.peers))
	copy(peersCopy, s.peers)
	return peersCopy
}

func (s *server) SendMessage(to PeerId, msg Message) error {
	s.peerLock.Lock()
	isConnected := slices.Contains(s.peers, to)
	s.peerLock.Unlock()

	if !isConnected {
		return fmt.Errorf("cannot send message to peer %s: not connected", to)
	}
	return s.network.transferMessage(s.id, to, msg)
}

func (s *server) RegisterMessageHandler(handler MessageHandler) {
	s.handlerLock.Lock()
	defer s.handlerLock.Unlock()
	s.handlers = append(s.handlers, handler)
}

func (s *server) receiveMessage(from PeerId, msg Message) {
	s.handlerLock.Lock()
	defer s.handlerLock.Unlock()
	for _, handler := range s.handlers {
		go handler.HandleMessage(from, msg)
	}
}

// connectTo is an internal method called by the Network to add a peer
// connection.
func (s *server) connectTo(peerId PeerId) {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()
	if !slices.Contains(s.peers, peerId) {
		s.peers = append(s.peers, peerId)
	}
}

// clearConnections is an internal method called by the Network to remove all
// peer connections, typically before applying a new topology.
func (s *server) clearConnections() {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()
	s.peers = []PeerId{}
}
