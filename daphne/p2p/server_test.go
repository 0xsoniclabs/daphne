package p2p

import (
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestServer_GetPeers_InitiallyThereAreNoPeers(t *testing.T) {
	require := require.New(t)
	id := PeerId("server1")

	network := NewNetwork()
	server, err := network.NewServer(id)
	require.NoError(err)

	peers := server.GetPeers()
	require.Empty(peers, "Expected no peers initially")
}

func TestServer_SendMessage_SendingToNonConnectedPeerFails(t *testing.T) {
	require := require.New(t)
	id1 := PeerId("server1")
	id2 := PeerId("server2")

	network := NewNetwork()
	server1, err := network.NewServer(id1)
	require.NoError(err)

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	err = server1.SendMessage(id2, msg)
	require.Error(err, "Expected error when sending to non-connected peer")
	require.EqualError(err, "cannot send message to peer server2: not connected")
}

func TestServer_receiveMessage_DeliversCallbacksAsynchronously(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		id := PeerId("sender")
		server := &server{}

		msg := Message{
			Code:    MessageCode_UnitTestProtocol_Ping,
			Payload: "ping",
		}

		block := make(chan struct{})
		done := make(chan struct{})
		messageHandler := NewMockMessageHandler(ctrl)
		messageHandler.EXPECT().HandleMessage(id, msg).Do(func(from PeerId, msg Message) {
			<-block
			close(done)
		})
		server.RegisterMessageHandler(messageHandler)

		server.receiveMessage(id, msg)
		close(block)
		<-done

		synctest.Wait()
	})
}
