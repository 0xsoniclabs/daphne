package p2p

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestServer_NewServer_FailsToConnect_AlreadyConnectedServer(t *testing.T) {
	require := require.New(t)
	id := PeerId("server1")

	network := NewNetwork()
	_, err := NewServer(id, network)
	require.NoError(err)

	_, err = NewServer(id, network)
	require.ErrorContains(err, fmt.Sprintf("server with ID %s already exists", id))
}

func TestServer_GetConnectedPeers_InitiallyThereAreNoPeers(t *testing.T) {
	require := require.New(t)
	id := PeerId("server1")

	network := NewNetwork()
	server, err := NewServer(id, network)
	require.NoError(err)

	peers := server.GetConnectedPeers()
	require.Empty(peers, "Expected no peers initially")
}

func TestServer_SendMessage_SendingToNonConnectedPeerFails(t *testing.T) {
	require := require.New(t)
	id1 := PeerId("server1")
	id2 := PeerId("server2")

	network := NewNetwork()
	server1, err := NewServer(id1, network)
	require.NoError(err)

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	err = server1.SendMessage(id2, msg)
	require.Error(err, "Expected error when sending to non-connected peer")
	require.EqualError(err, "cannot send message to peer server2: not connected")
}

func TestServer_RegisterMessageHandler_ReceivedMessagesAreForwardedToMultipleMessageHandlers(t *testing.T) {
	require := require.New(t)

	network := NewNetwork()
	id1 := PeerId("server1")
	id2 := PeerId("server2")

	server1, err := NewServer(id1, network)
	require.NoError(err)
	server2, err := NewServer(id2, network)
	require.NoError(err)

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	ctrl := gomock.NewController(t)
	receiver1 := NewMockMessageHandler(ctrl)
	receiver1.EXPECT().HandleMessage(id1, msg)
	receiver2 := NewMockMessageHandler(ctrl)
	receiver2.EXPECT().HandleMessage(id1, msg)
	server2.RegisterMessageHandler(receiver1)
	server2.RegisterMessageHandler(receiver2)

	err = server1.SendMessage(id2, msg)
	require.NoError(err)

	time.Sleep(10 * time.Millisecond)
}

func TestServer_Close_StopsListening(t *testing.T) {
	require := require.New(t)
	// when closing a server channels are closed
	// - no further messages can be received
	// - sender shall fails to receive

	network := NewNetwork()
	senderId := PeerId("sender")
	receiverId := PeerId("receiver")

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	ctrl := gomock.NewController(t)
	receiver2 := NewMockMessageHandler(ctrl)
	receiver2.EXPECT().HandleMessage(senderId, msg)

	sender, err := NewServer(senderId, network)
	require.NoError(err)
	receiver, err := NewServer(receiverId, network)
	require.NoError(err)
	receiver.RegisterMessageHandler(receiver2)

	err = sender.SendMessage(receiverId, msg)
	require.NoError(err)

	receiver.Close()

	// closed server cannot receive, sender shall
	// be notified
	err = sender.SendMessage(receiverId, msg)
	require.Error(err)
	require.EqualError(err, "peer receiver is closed")
}

func TestServer_Send_FailWithFullReceivingChannel(t *testing.T) {
	require := require.New(t)

	network := NewNetwork()
	id1 := PeerId("server1")
	id2 := PeerId("server2")

	server1, err := NewServer(id1, network)
	require.NoError(err)
	server2, err := NewServer(id2, network)
	require.NoError(err)

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	ctrl := gomock.NewController(t)
	messageHandler := NewMockMessageHandler(ctrl)
	messageHandler.EXPECT().HandleMessage(id1, msg).Do(func(_ PeerId, _ Message) {
		time.Sleep(1 * time.Hour)
	})
	server2.RegisterMessageHandler(messageHandler)

	err = server1.SendMessage(id2, msg)
	require.NoError(err)

	for range connectionSize {
		err = server1.SendMessage(id2, msg)
		require.NoError(err)
	}

	err = server1.SendMessage(id2, msg)
	require.ErrorContains(err, "cannot send message to peer server2: channel is full")
}

func TestServer_Close_ConsumesAllSentMessagesBeforeClosing(t *testing.T) {
	require := require.New(t)

	network := NewNetwork()
	id1 := PeerId("server1")
	id2 := PeerId("server2")

	server1, err := NewServer(id1, network)
	require.NoError(err)
	server2, err := NewServer(id2, network)
	require.NoError(err)

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	ctrl := gomock.NewController(t)
	messageHandler := NewMockMessageHandler(ctrl)
	// First message triggers expectation with wait, this allows to queue
	// more messages in the channel and close the server from the main test goroutine
	messageHandler.EXPECT().HandleMessage(id1, msg).Do(func(_ PeerId, _ Message) {
		time.Sleep(100 * time.Millisecond)
	})
	// 100 extra messages will be expected nevertheless
	messageHandler.EXPECT().HandleMessage(id1, msg).Times(100)
	server2.RegisterMessageHandler(messageHandler)

	// fist message (triggers wait)
	err = server1.SendMessage(id2, msg)
	require.NoError(err)

	// 100 extra messages
	for range 100 {
		err = server1.SendMessage(id2, msg)
		require.NoError(err)
	}

	server1.Close()
	server2.Close()

	// wait for goroutines to finish
	time.Sleep(100 * time.Millisecond)
}

func TestServer_GetConnectedPeers_DoesNotReturnItself(t *testing.T) {
	require := require.New(t)

	network := NewNetwork()
	server, err := NewServer(PeerId("server1"), network)
	require.NoError(err)

	peers := server.GetConnectedPeers()
	require.Empty(peers)

	server2, err := NewServer(PeerId("server2"), network)
	require.NoError(err)

	peers = server.GetConnectedPeers()
	require.NotContains(peers, server.GetLocalId(), "Expected GetConnectedPeers not to return itself")
	require.Contains(peers, server2.GetLocalId())
}
