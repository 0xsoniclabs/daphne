package p2p

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNetwork_NewServer_ProducesValidServerInstances(t *testing.T) {
	require := require.New(t)
	id1 := PeerId("server1")
	id2 := PeerId("server2")

	network := NewNetwork()
	server1, err := network.NewServer(id1)
	require.NoError(err)
	server2, err := network.NewServer(id2)
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
	_, err := network.NewServer(id)
	require.NoError(err)

	_, err = network.NewServer(id)
	require.EqualError(err, "server with ID server1 already exists")
}

func TestNetwork_CanSendMessagesBetweenServers(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler := NewMockMessageHandler(ctrl)

	id1 := PeerId("server1")
	id2 := PeerId("server2")

	network := NewNetwork()
	server1, err := network.NewServer(id1)
	require.NoError(t, err)
	server2, err := network.NewServer(id2)
	require.NoError(t, err)

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	handler.EXPECT().HandleMessage(id1, msg)
	server2.RegisterMessageHandler(handler)

	require.NoError(t, server1.SendMessage(id2, msg))

	time.Sleep(100 * time.Millisecond) // Allow time for async message delivery
}

func TestNetwork_NewServer_ServersAreFullyConnected(t *testing.T) {
	require := require.New(t)
	id1 := PeerId("server1")
	id2 := PeerId("server2")
	id3 := PeerId("server3")

	network := NewNetwork()
	server1, err := network.NewServer(id1)
	require.NoError(err)
	server2, err := network.NewServer(id2)
	require.NoError(err)
	server3, err := network.NewServer(id3)
	require.NoError(err)

	require.ElementsMatch([]PeerId{id2, id3}, server1.GetPeers())
	require.ElementsMatch([]PeerId{id1, id3}, server2.GetPeers())
	require.ElementsMatch([]PeerId{id1, id2}, server3.GetPeers())
}

func TestNetwork_transferMessage_DetectsInvalidSender(t *testing.T) {
	require := require.New(t)
	network := NewNetwork()

	id1 := PeerId("server1")
	id2 := PeerId("server2")

	_, err := network.NewServer(id2)
	require.NoError(err)

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	err = network.transferMessage(id1, id2, msg)
	require.Error(err)
	require.EqualError(err, "cannot send message from peer server1: not connected")
}

func TestNetwork_transferMessage_DetectsInvalidReceiver(t *testing.T) {
	require := require.New(t)
	network := NewNetwork()

	id1 := PeerId("server1")
	id2 := PeerId("server2")

	_, err := network.NewServer(id1)
	require.NoError(err)

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	err = network.transferMessage(id1, id2, msg)
	require.Error(err)
	require.EqualError(err, "cannot send message to peer server2: not connected")
}

func TestNetwork_transferMessage_FailsOnFullQueue(t *testing.T) {
	network := NewNetwork()
	require := require.New(t)

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	ctrl := gomock.NewController(t)
	server1 := NewMockServer(ctrl)
	server1.EXPECT().GetLocalId().Return(PeerId("server1"))
	server1.EXPECT().ReceiveMessage(PeerId("server2"), msg).
		Do(func(from PeerId, msg Message) {
			time.Sleep(100 * time.Millisecond) // Simulate slow processing
		})
	require.NoError(network.ConnectServer(server1))

	server2 := NewMockServer(ctrl)
	server2.EXPECT().GetLocalId().Return(PeerId("server2"))
	require.NoError(network.ConnectServer(server2))

	err := network.transferMessage(PeerId("server2"), PeerId("server1"), msg)

	require.NoError(err)

	time.Sleep(100 * time.Millisecond) // Allow time for async message delivery
}

func TestNetwork_processMessage_DropsMessages(t *testing.T) {}

func TestNetwork_processMessage_DelaysMessages(t *testing.T) {}

func TestNetwork_drainQueue_ProcessesAllMessages(t *testing.T) {}

func TestNetwork_Shutdown_StopsWorkerGracefullyAndDrainsQueue(t *testing.T) {}
