package generic

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// testReceiver is a simple implementation of BroadcastReceiver for testing purposes.
type testReceiver struct {
	f func(message uint32)
}

func (r *testReceiver) OnMessage(message uint32) {
	if r.f != nil {
		r.f(message)
	}
}

func Test_NewGossip(t *testing.T) {
	// Create a mock P2P server.
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// Expect the RegisterMessageHandler to be called once, for the gossip instance.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).Times(1)

	// Create a new gossip instance
	_ = NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	}, p2p.MessageCode_TxGossip_NewTransaction)
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
	}, p2p.MessageCode_TxGossip_NewTransaction)

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
	}, p2p.MessageCode_TxGossip_NewTransaction)

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
	}, p2p.MessageCode_TxGossip_NewTransaction)

	// Broadcast a message that will fail to send.
	gossip.Broadcast(uint32(1))

	// Check if the message is known to the peer.
	known := gossip.isMessageKnownByPeer(p2p.PeerId("peer1"), uint32(1))
	require.False(t, known, "Message should not be marked as known after a failed send")
}

func Test_RegisterReceiver(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	gossip := NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	}, p2p.MessageCode_TxGossip_NewTransaction)

	for range 3 {
		gossip.RegisterReceiver(&testReceiver{})
	}
	require.Len(t, gossip.receivers, 3, "Should be able to register multiple receivers")
}

func Test_Gossip_HandleMessage_OnMessageIsCalledOnAllReceivers(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{}).AnyTimes()

	gossip := NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	}, p2p.MessageCode_TxGossip_NewTransaction)

	// Create a test receiver that adds to a list when it receivers a message.
	receiverOnMessageList := make([]string, 0)
	for i := range 3 {
		receiver := &testReceiver{
			f: func(message uint32) {
				receiverOnMessageList = append(receiverOnMessageList, fmt.Sprintf("%d-%d", i, message))
			},
		}
		gossip.RegisterReceiver(receiver)
	}

	// Handle a message.
	gossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	})

	require.ElementsMatch(t, receiverOnMessageList, []string{"0-1", "1-1", "2-1"},
		"All receivers should have received the same message")
}

func Test_Gossip_HandleMessage_InvalidCodeNoops(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Invalid code should not trigger a broadcast.
	p2pServer.EXPECT().GetPeers().Times(0)

	gossip := NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	}, p2p.MessageCode_TxGossip_NewTransaction)

	// Handle a message with an invalid code.
	gossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    -123, // Invalid code
		Payload: uint32(1),
	})
}

func Test_Gossip_HandleMessage_InvalidPayloadNoops(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Invalid payload should not trigger a broadcast.
	p2pServer.EXPECT().GetPeers().Times(0)

	gossip := NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	}, p2p.MessageCode_TxGossip_NewTransaction)

	// Handle a message with an invalid payload type.
	gossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: "invalid", // Invalid payload type
	})
}

func Test_Gossip_HandleMessage_ReceivingAMessageSetsItAsKnownBySender(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Server has one peer.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1"}).Times(1)

	gossip := NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	}, p2p.MessageCode_TxGossip_NewTransaction)

	// Handle a message.
	gossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	})

	// Check if the message is known to the sender.
	known := gossip.isMessageKnownByPeer(p2p.PeerId("peer1"), uint32(1))
	require.True(t, known, "Message should be marked as known after being received")
}

func Test_Gossip_HandleMessage_MessageGetsBroadcast(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Server has two peers.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"}).Times(1)
	// Expect SendMessage to be called for peer2.
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer2"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	}).Return(nil).Times(1)

	gossip := NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	}, p2p.MessageCode_TxGossip_NewTransaction)

	// Handle a message.
	gossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	})
}
