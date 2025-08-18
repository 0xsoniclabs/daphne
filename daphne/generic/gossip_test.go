package generic

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_NewGossip(t *testing.T) {
	// Create a mock P2P server.
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// Expect the RegisterMessageHandler to be called once, for the gossip instance.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).Times(1)

	// Create a new gossip instance
	_ = NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	})
}

func Test_Gossip_Broadcast_AllPeersReceiveMessage(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))

	// Server has two peers.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"}).AnyTimes()
	// Expect the SendMessage to be called for each peer.
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	}).Return(nil).Times(1)
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer2"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	}).Return(nil).Times(1)
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	gossip := NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	})

	// Broadcast a message.
	gossip.Broadcast(uint32(1))
}

func Test_Gossip_Broadcast_BroadcastingMessageKnownToPeerDoesNotSendMessage(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))

	// Server has one peer.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1"}).AnyTimes()
	// Expect the SendMessage to be called once for both messages.
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	}).Return(nil).Times(1)
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(2),
	}).Return(nil).Times(1)
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	gossip := NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	})

	// Broadcast a message that is known to the peer (this will send the message).
	gossip.Broadcast(uint32(1))
	// Broadcast the message again, which is now known to the peer
	// (this will not send the message).
	gossip.Broadcast(uint32(1))
	// Broadcast a new unknown message to the same peer.
	gossip.Broadcast(uint32(2))
}

func Test_Gossip_Broadcast_BroadcastErrorDoesNotMarkMessageAsKnown(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))

	// Server has one peer.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1"}).AnyTimes()
	// Expect the SendMessage to return an error.
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	}).Return(fmt.Errorf("send error")).Times(1)
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	gossip := NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	})

	// Broadcast a message that will fail to send.
	gossip.Broadcast(uint32(1))

	// Check if the message is known to the peer.
	known := gossip.isTransactionKnownByPeer(p2p.PeerId("peer1"), uint32(1))
	require.False(t, known, "Message should not be marked as known after a failed send")
}
