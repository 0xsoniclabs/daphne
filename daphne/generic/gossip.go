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
		p2pServer:                p2pServer,
		extractKeyFromMessage:    extractKeyFromMessage,
		transactionsKnownByPeers: make(map[p2p.PeerId]map[K]struct{}),
		expectedMessageCode:      expectedMessageCode,
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
	// transactionsKnownByPeers keeps track of which transactions are known by which peers.
	// This is used to avoid sending the same transaction to the same peer multiple times.
	// The map is thread-safe.
	transactionsKnownByPeers      map[p2p.PeerId]map[K]struct{}
	transactionsKnownByPeersMutex sync.Mutex
	// receivers is a list of receivers that the messages will be broadcast to.
	receivers []BroadcastReceiver[M]
	// expectedMessageCode is the code of the message that this gossip instance handles.
	expectedMessageCode p2p.MessageCode
}

func (g *gossip[K, M]) Broadcast(message M) {
	for _, peer := range g.p2pServer.GetPeers() {
		if g.isTransactionKnownByPeer(peer, message) {
			continue
		}
		err := g.p2pServer.SendMessage(peer, p2p.Message{
			Code:    g.expectedMessageCode,
			Payload: message,
		})
		if err != nil {
			slog.Warn("Failed to send transaction gossip", "sender", peer,
				"transaction hash", g.extractKeyFromMessage(message), "error", err)
			continue
		}
		g.markTransactionKnownByPeer(peer, message)
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
	if _, exists := g.transactionsKnownByPeers[from]; !exists {
		g.transactionsKnownByPeers[from] = make(map[K]struct{})
	}
	g.transactionsKnownByPeers[from][key] = struct{}{}

	g.Broadcast(incoming)

	for _, receiver := range g.receivers {
		receiver.OnMessage(incoming)
	}
}

// isTransactionKnownByPeer returns true if the transaction
// is new for the specified peer. It is thread-safe.
func (g *gossip[K, M]) isTransactionKnownByPeer(peer p2p.PeerId, message M) bool {
	g.transactionsKnownByPeersMutex.Lock()
	defer g.transactionsKnownByPeersMutex.Unlock()
	if _, exists := g.transactionsKnownByPeers[peer]; !exists {
		return false
	}
	if _, exists := g.transactionsKnownByPeers[peer][g.extractKeyFromMessage(message)]; !exists {
		return false
	}
	return true
}

// markTransactionKnownByPeer updates the known transactions for a
// specified peer. It is thread-safe.
func (g *gossip[K, M]) markTransactionKnownByPeer(peer p2p.PeerId, message M) {
	g.transactionsKnownByPeersMutex.Lock()
	defer g.transactionsKnownByPeersMutex.Unlock()
	if _, exists := g.transactionsKnownByPeers[peer]; !exists {
		g.transactionsKnownByPeers[peer] = make(map[K]struct{})
	}
	if _, exists := g.transactionsKnownByPeers[peer][g.extractKeyFromMessage(message)]; !exists {
		g.transactionsKnownByPeers[peer][g.extractKeyFromMessage(message)] = struct{}{}
	}
}
