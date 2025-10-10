package generic

import (
	"log/slog"
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
)

// NewGossip creates a new Gossip instance.
// p2pServer is the P2P server used to send and receive messages.
// extractKeyFromMessage is a function that extracts a key from a message,
// used for lookup. Keys should be unique for each message.
// expectedMessageCode is the code of the message that this gossip instance
// handles.
func NewGossip[K comparable, M any](
	p2pServer p2p.Server,
	extractKeyFromMessage func(M) K,
	expectedMessageCode p2p.MessageCode,
) *gossip[K, M] {
	return NewGossipWithStrategy(
		p2pServer,
		extractKeyFromMessage,
		expectedMessageCode,
		NewFloodFallbackStrategy[K](),
	)
}

// NewGossipWithStrategy creates a new Gossip instance with a custom
// GossipStrategy, which is used to control gossip behavior.
func NewGossipWithStrategy[K comparable, M any](
	p2pServer p2p.Server,
	extractKeyFromMessage func(M) K,
	expectedMessageCode p2p.MessageCode,
	strategy GossipStrategy[K],
) *gossip[K, M] {
	res := &gossip[K, M]{
		p2pServer:             p2pServer,
		extractKeyFromMessage: extractKeyFromMessage,
		expectedMessageCode:   expectedMessageCode,
		strategy:              strategy,
	}
	p2pServer.RegisterMessageHandler(p2p.WrapMessageHandler(res.handleMessage))
	return res
}

// gossip is the concrete implementation of the Gossip interface.
type gossip[K comparable, M any] struct {
	p2pServer             p2p.Server
	extractKeyFromMessage func(M) K
	// strategy controls when messages should be gossiped to peers
	strategy GossipStrategy[K]
	// receivers is a list of receivers that the messages will be broadcast to.
	receivers           []BroadcastReceiver[M]
	receiversMutex      sync.Mutex
	expectedMessageCode p2p.MessageCode
}

func (g *gossip[K, M]) Broadcast(message M) {
	messageKey := g.extractKeyFromMessage(message)
	selfId := g.p2pServer.GetLocalId()

	if g.strategy.ShouldGossip(selfId, messageKey) {
		g.strategy.OnSent(selfId, messageKey)
		g.receiversMutex.Lock()
		for _, receiver := range g.receivers {
			go receiver.OnMessage(message)
		}
		g.receiversMutex.Unlock()
	}

	for _, peer := range g.p2pServer.GetPeers() {
		if !g.strategy.ShouldGossip(peer, messageKey) {
			continue
		}

		err := g.p2pServer.SendMessage(peer, p2p.Message{
			Code:    g.expectedMessageCode,
			Payload: message,
		})
		if err != nil {
			slog.Warn("Failed to send message gossip", "sender", peer,
				"message key", messageKey, "error", err)
			g.strategy.OnSendFailed(peer, messageKey, err)
			continue
		}
		g.strategy.OnSent(peer, messageKey)
	}
}

func (g *gossip[K, M]) RegisterReceiver(receiver BroadcastReceiver[M]) {
	g.receiversMutex.Lock()
	defer g.receiversMutex.Unlock()
	g.receivers = append(g.receivers, receiver)
}

func (g *gossip[K, M]) UnregisterReceiver(receiver BroadcastReceiver[M]) {
	g.receiversMutex.Lock()
	defer g.receiversMutex.Unlock()
	g.receivers = slices.DeleteFunc(g.receivers, func(r BroadcastReceiver[M]) bool {
		return r == receiver
	})
}

func (g *gossip[K, M]) handleMessage(from p2p.PeerId, msg p2p.Message) {
	if msg.Code != g.expectedMessageCode {
		return
	}
	incoming, ok := msg.Payload.(M)
	if !ok {
		slog.Warn("Received invalid message payload", "payload", msg.Payload)
		return
	}

	g.strategy.OnReceived(from, g.extractKeyFromMessage(incoming))

	g.Broadcast(incoming)
}
