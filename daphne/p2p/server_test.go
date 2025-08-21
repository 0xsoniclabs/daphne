package p2p

import (
	"testing"

	"github.com/stretchr/testify/require"
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

func TestServer_NewServer_ShutsDownCorrectly(t *testing.T) {
	require := require.New(t)
	id := PeerId("server1")

	network := NewNetwork()
	server, err := network.NewServer(id)
	require.NoError(err)

	// Ensure the server is running
	require.NotNil(server)

	// Shutdown the network
	network.Shutdown()

	// Attempt to send a message after shutdown
	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}
	err = server.SendMessage(id, msg)
	require.Error(err, "Expected error when sending message after shutdown")
	require.EqualError(err, "network is shutting down")
}
