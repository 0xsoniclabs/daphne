package generic

import (
	"fmt"
	"sync"
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
	gossip := NewGossip(p2pServer, testExtractKeyFromMessage,
		p2p.MessageCode_UnitTestProtocol_Ping)
	require.NotNil(t, gossip, "Gossip instance should not be nil")
	require.Equal(t, p2p.MessageCode_UnitTestProtocol_Ping, gossip.expectedMessageCode,
		"Expected message code should match the one provided during initialization")
	require.NotNil(t, gossip.extractKeyFromMessage,
		"Extract key function should not be nil")
	require.Equal(t, p2pServer, gossip.p2pServer,
		"P2P server should match the one provided during initialization")
	require.NotNil(t, gossip.messagesKnownByPeers,
		"Messages known by peers map should not be nil")
	require.Empty(t, gossip.messagesKnownByPeers,
		"Messages known by peers map should be empty on initialization")
}

func Test_Gossip_Broadcast_AllPeersReceiveMessage(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()

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

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

	// Broadcast a message.
	gossip.Broadcast(uint32(1))
}

func Test_Gossip_Broadcast_BroadcastingMessageKnownToPeerDoesNotSendMessage(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()

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

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

	// Broadcast a message that is known to the peer (this will send the message).
	gossip.Broadcast(uint32(1))
	// Broadcast the message again, which is now known to the peer
	// (this will not send the message).
	gossip.Broadcast(uint32(1))
	// Broadcast a new unknown message to the same peer.
	gossip.Broadcast(uint32(2))
}

func Test_Gossip_Broadcast_BroadcastErrorDoesMarkMessageAsKnown(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()

	// Server has one peer.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1"}).AnyTimes()
	// Expect the SendMessage to return an error.
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	}).Return(fmt.Errorf("send error")).Times(1)
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

	// Broadcast a message that will fail to send.
	gossip.Broadcast(uint32(1))

	// Check if the message is known to the peer.
	known := gossip.isMessageKnownByPeer(p2p.PeerId("peer1"), uint32(1))
	require.True(t, known, "Message should still be marked as known after a failed send")
}

func Test_RegisterReceiver(t *testing.T) {
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

	for range 3 {
		receiver := NewMockBroadcastReceiver[uint32](ctrl)
		gossip.RegisterReceiver(receiver)
	}
	require.Len(t, gossip.receivers, 3, "Should be able to register multiple receivers")
}

func Test_Gossip_HandleMessage_OnMessageIsCalledOnAllReceivers(t *testing.T) {
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	// This method is irrelevant for the test.
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{}).AnyTimes()

	gossip := NewGossip(p2pServer, func(msg uint32) string {
		return fmt.Sprintf("%d", msg)
	}, p2p.MessageCode_TxGossip_NewTransaction)

	// Create a test receiver that adds to a list when it receivers a message.
	receiverOnMessageList := make([]string, 0)
	for i := range 3 {
		receiver := NewMockBroadcastReceiver[uint32](ctrl)
		receiver.EXPECT().OnMessage(gomock.Any()).DoAndReturn(
			func(message uint32) {
				receiverOnMessageList =
					append(receiverOnMessageList, fmt.Sprintf("%d-%d", i, message))
			}).AnyTimes()
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

func Test_Gossip_HandleMessage_InvalidCodeIsIgnored(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Invalid code should not trigger a broadcast.
	p2pServer.EXPECT().GetPeers().Times(0)

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

	// Handle a message with an invalid code.
	gossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    -123, // Invalid code
		Payload: uint32(1),
	})
}

func Test_Gossip_HandleMessage_InvalidPayloadIsIgnored(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Invalid payload should not trigger a broadcast.
	p2pServer.EXPECT().GetPeers().Times(0)

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

	// Handle a message with an invalid payload type.
	gossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: "invalid", // Invalid payload type
	})
}

func Test_Gossip_HandleMessage_ReceivingAMessageSetsItAsKnownBySender(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// This method is irrelevant for the test.
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Server has one peer.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1"}).Times(1)

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

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
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Server has two peers.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"}).Times(1)
	// Expect SendMessage to be called for peer2.
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer2"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	}).Return(nil).Times(1)

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

	// Handle a message.
	gossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	})
}

func Test_Gossip_HandleMessage_IsThreadSafe(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// These methods are irrelevant for the test.
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	p2pServer.EXPECT().SendMessage(gomock.Any(), gomock.Any()).AnyTimes()
	// Server has two peers.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"}).AnyTimes()

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

	wg := sync.WaitGroup{}
	wg.Add(4)

	// Handle messages concurrently.
	go func() {
		defer wg.Done()
		gossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
			Code:    p2p.MessageCode_TxGossip_NewTransaction,
			Payload: uint32(1),
		})
	}()
	// Same message from another peer.
	go func() {
		defer wg.Done()
		gossip.HandleMessage(p2p.PeerId("peer2"), p2p.Message{
			Code:    p2p.MessageCode_TxGossip_NewTransaction,
			Payload: uint32(1),
		})
	}()
	// Same peer sending a different message.
	go func() {
		defer wg.Done()
		gossip.HandleMessage(p2p.PeerId("peer1"), p2p.Message{
			Code:    p2p.MessageCode_TxGossip_NewTransaction,
			Payload: uint32(2),
		})
	}()
}

// testExtractKeyFromMessage is an auxiliary function that is a placeholder
// for extracting a key from a message. It converts the uint32 message to a string.
func testExtractKeyFromMessage(msg uint32) string {
	return fmt.Sprintf("%d", msg)
}

func TestGossip_Broadcast_ReachesSender(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	server.EXPECT().GetLocalId().Return(p2p.PeerId("sender")).AnyTimes()
	server.EXPECT().GetPeers().Return([]p2p.PeerId{}).AnyTimes()

	gossip := NewGossip(server, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

	receiver := NewMockBroadcastReceiver[uint32](ctrl)
	receiver.EXPECT().OnMessage(uint32(1)).Times(1)

	gossip.RegisterReceiver(receiver)
	gossip.Broadcast(uint32(1))
}

func TestGossip_OnMessage_MessagesAreOnlyDeliveredOnceToReceivers(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	server.EXPECT().GetLocalId().Return(p2p.PeerId("sender")).AnyTimes()
	server.EXPECT().GetPeers().Return([]p2p.PeerId{}).AnyTimes()

	gossip := NewGossip(server, testExtractKeyFromMessage,
		p2p.MessageCode_TxGossip_NewTransaction)

	receiver := NewMockBroadcastReceiver[uint32](ctrl)
	receiver.EXPECT().OnMessage(uint32(1)).Times(1)

	msg := p2p.Message{
		Code:    p2p.MessageCode_TxGossip_NewTransaction,
		Payload: uint32(1),
	}

	gossip.RegisterReceiver(receiver)
	gossip.HandleMessage(p2p.PeerId("A"), msg)
	gossip.HandleMessage(p2p.PeerId("B"), msg)
}
