package generic

import (
	"fmt"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGossip_NewGossip_InstantiatesGossipAndDefaultsToFloodFallbackStrategy(t *testing.T) {
	// Create a mock P2P server.
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// Expect the RegisterMessageHandler to be called once, for the gossip instance.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any())

	// Create a new gossip instance
	gossip := NewGossip(p2pServer, testExtractKeyFromMessage)
	require.NotNil(t, gossip, "Gossip instance should not be nil")
	require.NotNil(t, gossip.extractKeyFromMessage,
		"Extract key function should not be nil")
	require.Equal(t, p2pServer, gossip.p2pServer,
		"P2P server should match the one provided during initialization")
	require.NotNil(t, gossip.strategy,
		"Strategy should not be nil")
	require.IsType(t, &FloodFallbackStrategy[string]{}, gossip.strategy,
		"Default strategy should be FloodFallbackStrategy")
}

func TestGossip_Broadcast_CallsStrategyAndSendsToAllPeers(t *testing.T) {
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()

	// Server has two peers.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"}).AnyTimes()
	// Expect the SendMessage to be called for each peer.
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer1"), GossipMessage[uint32]{Payload: 1})
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer2"), GossipMessage[uint32]{Payload: 1})
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	strategy := NewMockGossipStrategy[string](ctrl)
	strategy.EXPECT().ShouldGossip(p2p.PeerId("self"), "1").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("self"), "1")
	strategy.EXPECT().ShouldGossip(p2p.PeerId("peer1"), "1").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("peer1"), "1")
	strategy.EXPECT().ShouldGossip(p2p.PeerId("peer2"), "1").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("peer2"), "1")

	gossip := NewGossipWithStrategy(p2pServer, testExtractKeyFromMessage, strategy)

	// Broadcast a message.
	gossip.Broadcast(uint32(1))
}

func TestGossip_Broadcast_RespectsStrategyDecisionToSkipPeers(t *testing.T) {
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()

	// Server has one peer.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1"}).AnyTimes()
	// Expect the SendMessage to be called once for both messages.
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer1"), GossipMessage[uint32]{Payload: 1})
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer1"), GossipMessage[uint32]{Payload: 2})
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	strategy := NewMockGossipStrategy[string](ctrl)
	// First broadcast of message 1
	strategy.EXPECT().ShouldGossip(p2p.PeerId("self"), "1").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("self"), "1")
	strategy.EXPECT().ShouldGossip(p2p.PeerId("peer1"), "1").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("peer1"), "1")
	// Second broadcast of message 1 - strategy says no
	strategy.EXPECT().ShouldGossip(p2p.PeerId("self"), "1").Return(false)
	strategy.EXPECT().ShouldGossip(p2p.PeerId("peer1"), "1").Return(false)
	// Broadcast of message 2
	strategy.EXPECT().ShouldGossip(p2p.PeerId("self"), "2").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("self"), "2")
	strategy.EXPECT().ShouldGossip(p2p.PeerId("peer1"), "2").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("peer1"), "2")

	gossip := NewGossipWithStrategy(p2pServer, testExtractKeyFromMessage, strategy)

	// Broadcast a message that is unknown to the peer (this will send the message).
	gossip.Broadcast(uint32(1))
	// Broadcast the message again, which is now known to the peer
	// (this will not send the message).
	gossip.Broadcast(uint32(1))
	// Broadcast a new unknown message to the same peer.
	gossip.Broadcast(uint32(2))
}

func TestGossip_Broadcast_CallsOnSendFailedWhenSendMessageFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()

	// Server has one peer.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1"}).AnyTimes()
	// Expect the SendMessage to return an error.
	sendError := fmt.Errorf("send error")
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer1"), GossipMessage[uint32]{Payload: 1}).Return(sendError)
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

	strategy := NewMockGossipStrategy[string](ctrl)
	strategy.EXPECT().ShouldGossip(p2p.PeerId("self"), "1").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("self"), "1")
	strategy.EXPECT().ShouldGossip(p2p.PeerId("peer1"), "1").Return(true)
	// Verify OnSendFailed is called with the error
	strategy.EXPECT().OnSendFailed(p2p.PeerId("peer1"), "1", sendError)

	gossip := NewGossipWithStrategy(p2pServer, testExtractKeyFromMessage, strategy)

	// Broadcast a message that will fail to send.
	gossip.Broadcast(uint32(1))
}

func TestGossip_RegisterReceiver_AddsReceiversToList(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		p2pServer := p2p.NewMockServer(ctrl)
		// This method is irrelevant for the test.
		p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
		p2pServer.EXPECT().GetPeers().Return(nil).AnyTimes()
		p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

		gossip := NewGossip(p2pServer, testExtractKeyFromMessage)

		receiver := NewMockBroadcastReceiver[uint32](ctrl)
		receiver.EXPECT().OnMessage(uint32(1))

		gossip.RegisterReceiver(receiver)
		gossip.handleMessage(p2p.PeerId("peer1"), GossipMessage[uint32]{Payload: 1})
		synctest.Wait()
	})
}

func TestGossip_UnregisterReceiver_RemovesReceiverFromList(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		p2pServer := p2p.NewMockServer(ctrl)
		// These methods are irrelevant for the test.
		p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		p2pServer.EXPECT().GetLocalId().AnyTimes()
		p2pServer.EXPECT().GetPeers().AnyTimes()

		gossip := NewGossip(p2pServer, testExtractKeyFromMessage)

		receivers := make([]*MockBroadcastReceiver[uint32], 0, 3)
		for i := range 3 {
			receiver := NewMockBroadcastReceiver[uint32](ctrl)
			if i == 1 {
				receiver.EXPECT().OnMessage(uint32(1)).Times(0)
			} else {
				receiver.EXPECT().OnMessage(uint32(1))
			}
			gossip.RegisterReceiver(receiver)
			receivers = append(receivers, receiver)
		}

		// Unregister the second receiver.
		gossip.UnregisterReceiver(receivers[1])
		gossip.Broadcast(uint32(1))
		synctest.Wait()
	})
}

func TestGossip_handleMessage_OnMessageIsCalledOnAllReceivers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		p2pServer := p2p.NewMockServer(ctrl)
		// This method is irrelevant for the test.
		p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
		p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{}).AnyTimes()

		gossip := NewGossip(p2pServer, func(msg uint32) string {
			return fmt.Sprintf("%d", msg)
		})

		// Create a test receiver that adds to a list when it receivers a message.
		receivedMessageListLock := sync.Mutex{}
		receiverOnMessageList := make([]string, 0)
		for i := range 3 {
			receiver := NewMockBroadcastReceiver[uint32](ctrl)
			receiver.EXPECT().OnMessage(gomock.Any()).DoAndReturn(
				func(message uint32) {
					receivedMessageListLock.Lock()
					defer receivedMessageListLock.Unlock()
					receiverOnMessageList =
						append(receiverOnMessageList, fmt.Sprintf("%d-%d", i, message))
				}).AnyTimes()
			gossip.RegisterReceiver(receiver)
		}

		// Handle a message.
		gossip.handleMessage(p2p.PeerId("peer1"), GossipMessage[uint32]{Payload: 1})

		synctest.Wait()

		require.ElementsMatch(t, receiverOnMessageList, []string{"0-1", "1-1", "2-1"},
			"All receivers should have received the same message")
	})
}

func TestGossip_handleMessage_InvalidMessageTypeIsIgnored(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// This method is irrelevant for the test.
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Invalid code should not trigger a broadcast.
	p2pServer.EXPECT().GetPeers().Times(0)

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage)

	// Handle a message with an invalid code.
	gossip.handleMessage(p2p.PeerId("peer1"), "invalid-message")
}

func TestGossip_handleMessage_CallsStrategyOnReceived(t *testing.T) {
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	// This method is irrelevant for the test.
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Server has one peer.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1"})

	strategy := NewMockGossipStrategy[string](ctrl)
	// Verify OnReceived is called for the sender
	strategy.EXPECT().OnReceived(p2p.PeerId("peer1"), "1")
	// handleMessage calls Broadcast, which checks self and peer1
	strategy.EXPECT().ShouldGossip(p2p.PeerId("self"), "1").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("self"), "1")
	strategy.EXPECT().ShouldGossip(p2p.PeerId("peer1"), "1").Return(false)

	gossip := NewGossipWithStrategy(p2pServer, testExtractKeyFromMessage, strategy)

	// Handle a message from peer1.
	gossip.handleMessage(p2p.PeerId("peer1"), GossipMessage[uint32]{Payload: 1})
}

func TestGossip_handleMessage_BroadcastsAfterReceiving(t *testing.T) {
	ctrl := gomock.NewController(t)
	p2pServer := p2p.NewMockServer(ctrl)
	// This method is irrelevant for the test.
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	// Server has two peers.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"})
	// Expect SendMessage to be called for peer2.
	p2pServer.EXPECT().SendMessage(p2p.PeerId("peer2"), GossipMessage[uint32]{Payload: 1})

	strategy := NewMockGossipStrategy[string](ctrl)
	// OnReceived is called for the sender
	strategy.EXPECT().OnReceived(p2p.PeerId("peer1"), "1")
	// Broadcast checks self and both peers
	strategy.EXPECT().ShouldGossip(p2p.PeerId("self"), "1").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("self"), "1")
	strategy.EXPECT().ShouldGossip(p2p.PeerId("peer1"), "1").Return(false)
	strategy.EXPECT().ShouldGossip(p2p.PeerId("peer2"), "1").Return(true)
	strategy.EXPECT().OnSent(p2p.PeerId("peer2"), "1")

	gossip := NewGossipWithStrategy(p2pServer, testExtractKeyFromMessage, strategy)

	// Handle a message from peer1.
	gossip.handleMessage(p2p.PeerId("peer1"), GossipMessage[uint32]{Payload: 1})
}

func TestGossip_handleMessage_IsThreadSafe(t *testing.T) {
	p2pServer := p2p.NewMockServer(gomock.NewController(t))
	// These methods are irrelevant for the test.
	p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
	p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	p2pServer.EXPECT().SendMessage(gomock.Any(), gomock.Any()).AnyTimes()
	// Server has two peers.
	p2pServer.EXPECT().GetPeers().Return([]p2p.PeerId{"peer1", "peer2"}).AnyTimes()

	gossip := NewGossip(p2pServer, testExtractKeyFromMessage)

	wg := sync.WaitGroup{}
	wg.Add(4)

	// Handle messages concurrently.
	go func() {
		defer wg.Done()
		gossip.handleMessage(p2p.PeerId("peer1"), GossipMessage[uint32]{Payload: 1})
	}()
	// Same message from another peer.
	go func() {
		defer wg.Done()
		gossip.handleMessage(p2p.PeerId("peer2"), GossipMessage[uint32]{Payload: 1})
	}()
	// Same peer sending a different message.
	go func() {
		defer wg.Done()
		gossip.handleMessage(p2p.PeerId("peer1"), GossipMessage[uint32]{Payload: 2})
	}()
}

// testExtractKeyFromMessage is an auxiliary function that is a placeholder
// for extracting a key from a message. It converts the uint32 message to a string.
func testExtractKeyFromMessage(msg uint32) string {
	return fmt.Sprintf("%d", msg)
}

func TestGossip_Broadcast_NotifiesLocalReceivers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		server.EXPECT().GetLocalId().Return(p2p.PeerId("sender")).AnyTimes()
		server.EXPECT().GetPeers().Return([]p2p.PeerId{}).AnyTimes()

		gossip := NewGossip(server, testExtractKeyFromMessage)

		receiver := NewMockBroadcastReceiver[uint32](ctrl)
		receiver.EXPECT().OnMessage(uint32(1))

		gossip.RegisterReceiver(receiver)
		gossip.Broadcast(uint32(1))
		synctest.Wait()
	})
}

func TestGossip_handleMessage_DeliversOnlyOnceToReceivers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		server.EXPECT().GetLocalId().Return(p2p.PeerId("sender")).AnyTimes()
		server.EXPECT().GetPeers().Return([]p2p.PeerId{}).AnyTimes()

		gossip := NewGossip(server, testExtractKeyFromMessage)

		receiver := NewMockBroadcastReceiver[uint32](ctrl)
		receiver.EXPECT().OnMessage(uint32(1))

		msg := GossipMessage[uint32]{Payload: 1}

		gossip.RegisterReceiver(receiver)
		gossip.handleMessage(p2p.PeerId("A"), msg)
		gossip.handleMessage(p2p.PeerId("B"), msg)

		synctest.Wait()
	})
}

func TestGossip_Broadcast_DeliversCallbacksAsynchronously(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	server.EXPECT().GetLocalId().Return(p2p.PeerId("sender")).AnyTimes()
	server.EXPECT().GetPeers().Return([]p2p.PeerId{}).AnyTimes()

	gossip := NewGossip(server, testExtractKeyFromMessage)

	quit := make(chan struct{})
	done := make(chan struct{})
	receiver := NewMockBroadcastReceiver[uint32](ctrl)
	receiver.EXPECT().OnMessage(uint32(1)).Do(func(uint32) {
		<-quit
		close(done)
	})

	gossip.RegisterReceiver(receiver)
	gossip.Broadcast(uint32(1))
	close(quit)
	<-done
}

func TestGossipMessage_ReportsReadableMessageType(t *testing.T) {
	msg := GossipMessage[types.Transaction]{}
	require.EqualValues(t, "GossipMessage[Transaction]", p2p.GetMessageType(msg))
}
