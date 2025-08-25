package p2p

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNetwork_NewServer_ProducesValidServerInstances(t *testing.T) {
	require := require.New(t)
	id1 := PeerId("server1")
	id2 := PeerId("server2")

	network := NewNetwork()
	server1, err := NewServer(id1, network)
	require.NoError(err)
	server2, err := NewServer(id2, network)
	require.NoError(err)

	require.NotNil(server1)
	require.NotNil(server2)
	require.Equal(id1, server1.GetLocalId())
	require.Equal(id2, server2.GetLocalId())
}

func TestNetwork_NewServer_DetectsIdDuplicates(t *testing.T) {
	require := require.New(t)
	id := PeerId("server1")

	network := NewNetwork()
	_, err := NewServer(id, network)
	require.NoError(err)

	_, err = NewServer(id, network)
	require.ErrorContains(err, "server with ID server1 already exists")
}

func TestNetwork_CanSendMessagesBetweenServers(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler := NewMockMessageHandler(ctrl)

	id1 := PeerId("server1")
	id2 := PeerId("server2")

	network := NewNetwork()
	server1, err := NewServer(id1, network)
	require.NoError(t, err)
	defer server1.Close()
	server2, err := NewServer(id2, network)
	require.NoError(t, err)
	defer server2.Close()

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	handler.EXPECT().HandleMessage(id1, msg)
	server2.RegisterMessageHandler(handler)

	require.NoError(t, server1.SendMessage(id2, msg))
}

func TestNetwork_NewServer_ServersAreFullyConnected(t *testing.T) {
	require := require.New(t)
	id1 := PeerId("server1")
	id2 := PeerId("server2")
	id3 := PeerId("server3")

	network := NewNetwork()
	server1, err := NewServer(id1, network)
	require.NoError(err)
	server2, err := NewServer(id2, network)
	require.NoError(err)
	server3, err := NewServer(id3, network)
	require.NoError(err)

	require.ElementsMatch([]PeerId{id2, id3}, server1.GetConnectedPeers())
	require.ElementsMatch([]PeerId{id1, id3}, server2.GetConnectedPeers())
	require.ElementsMatch([]PeerId{id1, id2}, server3.GetConnectedPeers())
}
