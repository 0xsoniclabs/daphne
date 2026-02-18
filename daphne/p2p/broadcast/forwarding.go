// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package broadcast

import (
	"log/slog"
	reflect "reflect"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

// NewForwarding creates a new broadcast channel using the forwarding strategy.
// Forwarding is an optimistic, best-effort broadcast strategy that aims to
// minimize the number of messages on the network. Each message send through
// the network includes a list of peers that are believed to already know
// about the message, so that peers do not forward the message to those peers.
// This minimizes redundant messages on the network, but does not guarantee
// that all peers will receive all messages, especially in the presence of
// network issues, peer churn, or malicious peers.
//
// This strategy is provided as a simple, low-overhead base-line strategy for
// the performance comparison of more sophisticated strategies. It is not
// recommended for production use, as it does not guarantee reliable message
// delivery.
func NewForwarding[K comparable, M any](
	p2pServer p2p.Server,
	extractKeyFromMessage func(M) K,
) Channel[M] {
	return newForwarding(p2pServer, extractKeyFromMessage)
}

// newForwarding is an internal version of NewForwarding returning the concrete
// type to simplify testing. NewForwarding is the public factory function,
// required to satisfy the BroadcasterFactory type.
func newForwarding[K comparable, M any](
	p2pServer p2p.Server,
	extractKeyFromMessage func(M) K,
) *forwarding[K, M] {
	res := forwarding[K, M]{
		p2pServer:             p2pServer,
		extractKeyFromMessage: extractKeyFromMessage,
	}
	p2pServer.RegisterMessageHandler(p2p.WrapMessageHandler(res.handleMessage))
	return &res
}

type forwarding[K comparable, M any] struct {
	p2pServer             p2p.Server
	extractKeyFromMessage func(M) K
	receivers             Receivers[M]
	seen                  sets.Set[K]
	seenMutex             sync.Mutex
}

// Broadcast sends the given message to all registered receivers for message
// type M on all nodes of the network -- including receivers on the local node.
// Delivery is best-effort and not guaranteed.
func (f *forwarding[K, M]) Broadcast(message M) {
	peers := f.p2pServer.GetPeers()
	seen := sets.New(peers...)
	seen.Add(f.p2pServer.GetLocalId())
	f.sendMessages(sets.New(peers...), ForwardingMessage[M]{
		Payload:  message,
		Notified: seen,
	})
	f.receivers.Deliver(message)
}

// Register adds the given receiver to the list of receivers that will be
// notified when new messages are received. Each message will be delivered to
// each registered receiver at most once.
func (f *forwarding[K, M]) Register(receiver Receiver[M]) {
	f.receivers.Register(receiver)
}

// Unregister removes the given receiver from the list of registered receivers.
func (f *forwarding[K, M]) Unregister(receiver Receiver[M]) {
	f.receivers.Unregister(receiver)
}

// handleMessage processes an incoming message from a peer.
func (f *forwarding[K, M]) handleMessage(_ p2p.PeerId, message p2p.Message) {
	if _, ok := message.(ForwardingMessage[M]); !ok {
		// not a forwarding message
		return
	}
	incoming := message.(ForwardingMessage[M])

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
	f.sendMessages(newPeers, ForwardingMessage[M]{
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

// sendMessages sends the given forwarding message to all peers in the given set.
// If sending to a peer fails, the peer is removed from the Notified set of the
// message to avoid forwarding the message to that peer in future sends.
func (f *forwarding[K, M]) sendMessages(
	peers sets.Set[p2p.PeerId],
	message ForwardingMessage[M],
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
				"Failed to send forwarding message",
				"sender", f.p2pServer.GetLocalId(),
				"receiver", peer,
				"error", err,
			)
			continue
		}
	}
}

// ForwardingMessage is the protocol envelope for a message to be forwarded. It
// is annotating the message with the set of peers which are believed to be
// notified about this message, so that peers do not forward the message to
// those.
type ForwardingMessage[M any] struct {
	Payload  M
	Notified sets.Set[p2p.PeerId]
}

// MessageType returns the message type of the ForwardingMessage, which includes
// the message type of the payload M in a human-readable format. This helps
// making this message type readable in analyses.
func (m ForwardingMessage[M]) MessageType() p2p.MessageType {
	var payload M
	return "ForwardingMessage[" + p2p.GetMessageType(payload) + "]"
}

// MessageSize returns the size of the payload message M.
func (m ForwardingMessage[M]) MessageSize() uint32 {
	setSize := uint32(reflect.TypeFor[sets.Set[p2p.PeerId]]().Size())
	for peer := range m.Notified.All() {
		setSize += uint32(len(peer))
	}
	return p2p.GetMessageSize(m.Payload) + setSize
}
