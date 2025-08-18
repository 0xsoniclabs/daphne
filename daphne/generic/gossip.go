package generic

import (
	"log/slog"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
)

func NewGossip[K comparable, M any](
	p2pServer p2p.Server,
	extractKeyFromMessage func(M) K,
	expectedMessageCode p2p.MessageCode,
) *gossip[K, M] {
	res := &gossip[K, M]{
		p2pServer:             p2pServer,
		extractKeyFromMessage: extractKeyFromMessage,
		messagesKnownByPeers:  make(map[p2p.PeerId]map[K]struct{}),
		expectedMessageCode:   expectedMessageCode,
	}
	p2pServer.RegisterMessageHandler(res)
	return res
}

type gossip[K comparable, M any] struct {
	// p2pServer is the P2P server used to send and receive messages.
	p2pServer p2p.Server
	// extractKeyFromMessage is a function that extracts a key from a message,
	// used for lookup. Keys should be unique for each message.
	extractKeyFromMessage func(M) K
	// messagesKnownByPeers keeps track of which messages are known by which peers.
	// This is used to avoid sending the same message to the same peer multiple times.
	// The map is thread-safe.
	messagesKnownByPeers      map[p2p.PeerId]map[K]struct{}
	messagesKnownByPeersMutex sync.Mutex
	// receivers is a list of receivers that the messages will be broadcast to.
	receivers []BroadcastReceiver[M]
	// expectedMessageCode is the code of the message that this gossip instance handles.
	expectedMessageCode p2p.MessageCode
}

func (g *gossip[K, M]) Broadcast(message M) {
	for _, peer := range g.p2pServer.GetPeers() {
		if g.isMessageKnownByPeer(peer, message) {
			continue
		}
		g.markMessageKnownByPeer(peer, message)
		err := g.p2pServer.SendMessage(peer, p2p.Message{
			Code:    g.expectedMessageCode,
			Payload: message,
		})
		if err != nil {
			slog.Warn("Failed to send message gossip", "sender", peer,
				"message key", g.extractKeyFromMessage(message), "error", err)
			// If we fail to send the message, we don't want to keep it as known
			g.unmarkMessageKnownByPeer(peer, message)
			continue
		}
	}
}

func (g *gossip[K, M]) RegisterReceiver(receiver BroadcastReceiver[M]) {
	g.receivers = append(g.receivers, receiver)
}

func (g *gossip[K, M]) HandleMessage(from p2p.PeerId, msg p2p.Message) {
	if msg.Code != g.expectedMessageCode {
		return
	}
	incoming, ok := msg.Payload.(M)
	if !ok {
		slog.Warn("Received invalid message payload", "payload", msg.Payload)
		return
	}

	key := g.extractKeyFromMessage(incoming)
	if _, exists := g.messagesKnownByPeers[from]; !exists {
		g.messagesKnownByPeers[from] = make(map[K]struct{})
	}
	g.messagesKnownByPeers[from][key] = struct{}{}

	g.Broadcast(incoming)

	for _, receiver := range g.receivers {
		receiver.OnMessage(incoming)
	}
}

// isMessageKnownByPeer returns true if the message
// is new for the specified peer. It is thread-safe.
func (g *gossip[K, M]) isMessageKnownByPeer(peer p2p.PeerId, message M) bool {
	g.messagesKnownByPeersMutex.Lock()
	defer g.messagesKnownByPeersMutex.Unlock()
	if _, exists := g.messagesKnownByPeers[peer]; !exists {
		return false
	}
	if _, exists := g.messagesKnownByPeers[peer][g.extractKeyFromMessage(message)]; !exists {
		return false
	}
	return true
}

// markMessageKnownByPeer updates the known messages for a
// specified peer. It is thread-safe.
func (g *gossip[K, M]) markMessageKnownByPeer(peer p2p.PeerId, message M) {
	g.messagesKnownByPeersMutex.Lock()
	defer g.messagesKnownByPeersMutex.Unlock()
	if _, exists := g.messagesKnownByPeers[peer]; !exists {
		g.messagesKnownByPeers[peer] = make(map[K]struct{})
	}
	if _, exists := g.messagesKnownByPeers[peer][g.extractKeyFromMessage(message)]; !exists {
		g.messagesKnownByPeers[peer][g.extractKeyFromMessage(message)] = struct{}{}
	}
}

// unmarkMessageKnownByPeer removes the known message for a
// specified peer. It is thread-safe.
func (g *gossip[K, M]) unmarkMessageKnownByPeer(peer p2p.PeerId, message M) {
	g.messagesKnownByPeersMutex.Lock()
	defer g.messagesKnownByPeersMutex.Unlock()
	if _, exists := g.messagesKnownByPeers[peer]; exists {
		delete(g.messagesKnownByPeers[peer], g.extractKeyFromMessage(message))
	}
}
