package broadcast

import (
	"log/slog"

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
) *gossip[K, M] {
	return NewGossipWithStrategy(
		p2pServer,
		extractKeyFromMessage,
		NewFloodFallbackStrategy[K](),
	)
}

// NewGossipWithStrategy creates a new Gossip instance with a custom
// GossipStrategy, which is used to control gossip behavior.
func NewGossipWithStrategy[K comparable, M any](
	p2pServer p2p.Server,
	extractKeyFromMessage func(M) K,
	strategy GossipStrategy[K],
) *gossip[K, M] {
	res := &gossip[K, M]{
		p2pServer:             p2pServer,
		extractKeyFromMessage: extractKeyFromMessage,
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
	strategy  GossipStrategy[K]
	receivers Receivers[M]
}

func (g *gossip[K, M]) Broadcast(message M) {
	messageKey := g.extractKeyFromMessage(message)
	selfId := g.p2pServer.GetLocalId()

	if g.strategy.ShouldGossip(selfId, messageKey) {
		g.strategy.OnSent(selfId, messageKey)
		g.receivers.Deliver(message)
	}

	// Create the message to be broadcasted in an envelope with the expected
	// message code and the gossip message code.
	msg := GossipMessage[M]{
		Payload: message,
	}

	for _, peer := range g.p2pServer.GetPeers() {
		if !g.strategy.ShouldGossip(peer, messageKey) {
			continue
		}

		err := g.p2pServer.SendMessage(peer, msg)
		if err != nil {
			slog.Warn("Failed to send message gossip", "sender", peer,
				"message key", messageKey, "error", err)
			g.strategy.OnSendFailed(peer, messageKey, err)
			continue
		}
		g.strategy.OnSent(peer, messageKey)
	}
}

func (g *gossip[K, M]) Register(receiver Receiver[M]) {
	g.receivers.Register(receiver)
}

func (g *gossip[K, M]) Unregister(receiver Receiver[M]) {
	g.receivers.Unregister(receiver)
}

func (g *gossip[K, M]) handleMessage(from p2p.PeerId, msg p2p.Message) {
	if _, ok := msg.(GossipMessage[M]); !ok {
		return
	}
	payload := msg.(GossipMessage[M]).Payload
	g.strategy.OnReceived(from, g.extractKeyFromMessage(payload))
	g.Broadcast(payload)
}

// GossipMessage is the protocol envelope for a message to be gossiped. It is
// used to differentiate messages of type M potentially transferred for other
// reasons over the network from messages of type M to be gossiped.
type GossipMessage[M any] struct {
	Payload M
}

// MessageType returns the message type of the GossipMessage, which includes
// the message type of the payload M in a human-readable format. This helps
// making this message type readable in analyses.
func (m GossipMessage[M]) MessageType() p2p.MessageType {
	var payload M
	return "GossipMessage[" + p2p.GetMessageType(payload) + "]"
}
