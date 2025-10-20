package broadcast

import (
	"log/slog"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

// NewFlooding creates a new broadcast channel using the flooding strategy.
// Flooding is an optimistic, best-effort broadcast strategy that aims to
// minimize the number of messages on the network. Each message send through
// the network includes a list of peers that are believed to already know
// about the message, so that peers do not forward the message to those peers.
// This minimizes redundant messages on the network, but does not guarantee
// that all peers will receive all messages, especially in the presence of
// network issues, peer churn, or malicious peers.
//
// This strategy is is provided as a simple, low-overhead base-line strategy for
// the performance comparison of more sophisticated strategies. It is not
// recommended for production use, as it does not guarantee reliable message
// delivery.
func NewFlooding[K comparable, M any](
	p2pServer p2p.Server,
	extractKeyFromMessage func(M) K,
) Channel[M] {
	return newFlooding(p2pServer, extractKeyFromMessage)
}

// newFlooding is an internal version of NewFlooding returning the concrete
// type to simplify testing. NewFlooding is the public factory function,
// required to satisfy the BroadcasterFactory type.
func newFlooding[K comparable, M any](
	p2pServer p2p.Server,
	extractKeyFromMessage func(M) K,
) *flooding[K, M] {
	res := flooding[K, M]{
		p2pServer:             p2pServer,
		extractKeyFromMessage: extractKeyFromMessage,
	}
	p2pServer.RegisterMessageHandler(p2p.WrapMessageHandler(res.handleMessage))
	return &res
}

type flooding[K comparable, M any] struct {
	p2pServer             p2p.Server
	extractKeyFromMessage func(M) K
	receivers             Receivers[M]
	seen                  sets.Set[K]
	seenMutex             sync.Mutex
}

// Broadcast sends the given message to all registered receivers for message
// type M on all nodes of the network -- including receivers on the local node.
// Delivery is best-effort and not guaranteed.
func (f *flooding[K, M]) Broadcast(message M) {
	peers := f.p2pServer.GetPeers()
	seen := sets.New(peers...)
	seen.Add(f.p2pServer.GetLocalId())
	f.sendMessages(sets.New(peers...), FloodingMessage[M]{
		Payload:  message,
		Notified: seen,
	})
	f.receivers.Deliver(message)
}

// Register adds the given receiver to the list of receivers that will be
// notified when new messages are received. Each message will be delivered to
// each registered receiver at most once.
func (f *flooding[K, M]) Register(receiver Receiver[M]) {
	f.receivers.Register(receiver)
}

// Unregister removes the given receiver from the list of registered receivers.
func (f *flooding[K, M]) Unregister(receiver Receiver[M]) {
	f.receivers.Unregister(receiver)
}

// handleMessage processes an incoming message from a peer.
func (f *flooding[K, M]) handleMessage(_ p2p.PeerId, message p2p.Message) {
	if _, ok := message.(FloodingMessage[M]); !ok {
		// not a flooding message
		return
	}
	incoming := message.(FloodingMessage[M])

	// Check if this message has been processed before.
	key := f.extractKeyFromMessage(incoming.Payload)
	f.seenMutex.Lock()
	if f.seen.Contains(key) {
		// Already seen this message
		f.seenMutex.Unlock()
		return
	}
	f.seen.Add(key)
	f.seenMutex.Unlock()

	// Forward the message to all peers except those reported to have seen it
	// already by the message.
	peers := f.p2pServer.GetPeers()
	newPeers := sets.Difference(sets.New(peers...), incoming.Notified)
	f.sendMessages(newPeers, FloodingMessage[M]{
		Payload: incoming.Payload,
		Notified: sets.Union(
			sets.New(f.p2pServer.GetLocalId()),
			incoming.Notified,
			newPeers,
		),
	})

	// Deliver the message to local receivers
	f.receivers.Deliver(incoming.Payload)
}

// sendMessages sends the given flooding message to all peers in the given set.
// If sending to a peer fails, the peer is removed from the Notified set of the
// message to avoid forwarding the message to that peer in future sends.
func (f *flooding[K, M]) sendMessages(
	peers sets.Set[p2p.PeerId],
	message FloodingMessage[M],
) {
	for peer := range peers.All() {
		err := f.p2pServer.SendMessage(peer, message)
		if err != nil {
			// We can not change the list of notified peers for messages that
			// have already been sent, but we can avoid including the peer to
			// which the send failed to be included in the Notified set for
			// future sends of this message.
			message.Notified.Remove(peer)
			slog.Warn(
				"Failed to send flooding message",
				"sender", f.p2pServer.GetLocalId(),
				"receiver", peer,
				"error", err,
			)
			continue
		}
	}
}

// FloodingMessage is the protocol envelope for a message to be flooded. It is
// annotating the message with the set of peers which are believed to be
// notified about this message, so that peers do not forward the message to
// those.
type FloodingMessage[M any] struct {
	Payload  M
	Notified sets.Set[p2p.PeerId]
}

// MessageType returns the message type of the FloodingMessage, which includes
// the message type of the payload M in a human-readable format. This helps
// making this message type readable in analyses.
func (m FloodingMessage[M]) MessageType() p2p.MessageType {
	var payload M
	return "FloodingMessage[" + p2p.GetMessageType(payload) + "]"
}
