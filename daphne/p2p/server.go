package p2p

import (
	"fmt"
	"maps"
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
	// GetConnectedPeers returns a list of peers this server is directly connected to.
	GetConnectedPeers() []PeerId
	// SendMessage sends the given message to the specified peer.
	SendMessage(to PeerId, msg Message) error
	// RegisterMessageHandler registers a new handler for incoming messages.
	// Every future message received by this node will be passed to this handler.
	RegisterMessageHandler(handler MessageHandler)
	// Close stops listening for incoming messages.
	Close()
}

// MessageHandler is an interface for handling messages received from peers.
type MessageHandler interface {
	HandleMessage(from PeerId, msg Message)
}

// --- Server implementation ---

type server struct {
	id               PeerId
	listeningToPeers map[PeerId]peerListener
	handlers         []MessageHandler
	network          *Network
	listenersMutex   sync.Mutex
}

// NewServer creates a new server on this P2P network with the given PeerId. The
// resulting server instance can be used by a node to interact with the network.
func NewServer(id PeerId, network *Network) (*server, error) {
	if _, exists := network.peers[id]; exists {
		return nil, fmt.Errorf("server with ID %s already exists", id)
	}
	res := &server{
		id:               id,
		listeningToPeers: make(map[PeerId]peerListener),
		handlers:         make([]MessageHandler, 0),
		network:          network,
	}
	network.peers[id] = res
	return res, nil
}

func (s *server) GetLocalId() PeerId {
	return s.id
}

// GetConnectedPeers returns a list of peers this server is directly connected to.
// The server's own ID is excluded.
func (s *server) GetConnectedPeers() []PeerId {
	// TODO: this assumes complete connectivity
	res := slices.Collect(maps.Keys(s.network.peers))
	return slices.DeleteFunc(res, func(key PeerId) bool {
		return key == s.id
	})
}

// SendMessage sends a message to a connected peer.
// If the peer is not connected, an error is returned.
// The message is sent asynchronously; if the peer's channel
// is full or closed, an error is returned.
func (s *server) SendMessage(to PeerId, msg Message) error {
	visiblePeers := s.GetConnectedPeers()
	visibleServers := make(map[PeerId]*server)
	maps.Copy(visibleServers, s.network.peers)
	maps.DeleteFunc(visibleServers,
		func(key PeerId, value *server) bool {
			return !slices.Contains(visiblePeers, key)
		})

	peer, exists := visibleServers[to]
	if !exists {
		return fmt.Errorf("cannot send message to peer %s: not connected", to)
	}
	channel, done := peer.getChannel(s.id)
	select {
	case _, ok := <-done:
		if !ok {
			return fmt.Errorf("peer %s is closed", to)
		}
	default:
	}

	select {
	case channel <- msg:
	default:
		return fmt.Errorf("cannot send message to peer %s: channel is full or closed", to)
	}
	return nil
}

// getChannel returns a one directional communication channel between two peers.
// if not channel has been established, a new one is created.
func (s *server) getChannel(from PeerId) (chan<- Message, <-chan struct{}) {
	s.listenersMutex.Lock()
	defer s.listenersMutex.Unlock()
	if _, exists := s.listeningToPeers[from]; !exists {
		s.listeningToPeers[from] = startListeningToPeer(s, from)
	}
	return s.listeningToPeers[from].channel, s.listeningToPeers[from].done
}

func (s *server) RegisterMessageHandler(handler MessageHandler) {
	s.handlers = append(s.handlers, handler)
}

// Close stops listening for incoming messages from all peers.
func (s *server) Close() {
	s.listenersMutex.Lock()
	defer s.listenersMutex.Unlock()
	for _, listener := range s.listeningToPeers {
		listener.Close()
	}
}

// peerListener listens on incoming messages from a peer.
type peerListener struct {
	from    PeerId
	channel chan Message
	done    chan struct{}
}

const connectionSize = 1000

func startListeningToPeer(server *server, receiveFrom PeerId) peerListener {
	channel := make(chan Message, connectionSize)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range channel {
			for _, handler := range server.handlers {
				handler.HandleMessage(receiveFrom, msg)
			}
		}
	}()
	return peerListener{
		from:    receiveFrom,
		channel: channel,
		done:    done,
	}
}

// Close stops listening for incoming messages from the peer.
func (listener *peerListener) Close() {
	close(listener.channel)
	<-listener.done
}
