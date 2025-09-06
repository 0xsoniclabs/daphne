package p2p

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
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

	network.WaitForDeliveryOfSentMessages()
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

	require.ElementsMatch([]PeerId{id1, id2, id3}, server1.GetPeers())
	require.ElementsMatch([]PeerId{id1, id2, id3}, server2.GetPeers())
	require.ElementsMatch([]PeerId{id1, id2, id3}, server3.GetPeers())
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

func TestNetwork_transferMessage_tracksMessageMilestones(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	tracker := tracker.NewMockTracker(ctrl)

	A := PeerId("server-A")
	B := PeerId("server-B")
	code := MessageCode_UnitTestProtocol_Ping

	// Tracking information is reported in order.
	gomock.InOrder(
		tracker.EXPECT().Track(mark.MsgSent,
			"id", uint64(1), "from", A, "to", B, "type", code,
		),
		tracker.EXPECT().Track(mark.MsgReceived,
			"id", uint64(1), "from", A, "to", B, "type", code,
		),
		tracker.EXPECT().Track(mark.MsgConsumed,
			"id", uint64(1), "from", A, "to", B, "type", code,
		),
	)

	gomock.InOrder(
		tracker.EXPECT().Track(mark.MsgSent,
			"id", uint64(2), "from", B, "to", A, "type", code,
		),
		tracker.EXPECT().Track(mark.MsgReceived,
			"id", uint64(2), "from", B, "to", A, "type", code,
		),
		tracker.EXPECT().Track(mark.MsgConsumed,
			"id", uint64(2), "from", B, "to", A, "type", code,
		),
	)

	network := NewNetworkBuilder().WithTracker(tracker).Build()
	_, err := network.NewServer(A)
	require.NoError(err)
	_, err = network.NewServer(B)
	require.NoError(err)

	msg := Message{
		Code:    code,
		Payload: "ping",
	}

	require.NoError(network.transferMessage(A, B, msg))
	require.NoError(network.transferMessage(B, A, msg))
	network.WaitForDeliveryOfSentMessages()
}

func TestNetwork_WaitForAllMessagesBeingDelivered_DoesNotTimeOut(t *testing.T) {
	require := require.New(t)

	senderId := PeerId("sender")
	receiverId := PeerId("receiver")

	tests := map[string]func(t *testing.T, network *Network){
		"carry on with empty network": func(t *testing.T, network *Network) {
		},
		"carry on without sent messages": func(t *testing.T, network *Network) {
			_, err := network.NewServer(senderId)
			require.NoError(err)
			_, err = network.NewServer(receiverId)
			require.NoError(err)
		},
		"waits for send message to complete": func(t *testing.T, network *Network) {
			_, err := network.NewServer(senderId)
			require.NoError(err)
			receiver, err := network.NewServer(receiverId)
			require.NoError(err)

			msg := Message{
				Code:    MessageCode_UnitTestProtocol_Ping,
				Payload: "ping",
			}

			ctrl := gomock.NewController(t)
			handler := NewMockMessageHandler(ctrl)
			handler.EXPECT().HandleMessage(senderId, msg)
			receiver.RegisterMessageHandler(handler)

			err = network.transferMessage(senderId, receiverId, msg)
			require.NoError(err)
		},
		"waits for multiple sends to complete": func(t *testing.T, network *Network) {
			_, err := network.NewServer(senderId)
			require.NoError(err)
			receiver, err := network.NewServer(receiverId)
			require.NoError(err)

			ctrl := gomock.NewController(t)
			handler := NewMockMessageHandler(ctrl)
			handler.EXPECT().HandleMessage(senderId, gomock.Any()).Times(3)
			receiver.RegisterMessageHandler(handler)

			err = network.transferMessage(senderId, receiverId,
				Message{
					Code:    MessageCode_UnitTestProtocol_Ping,
					Payload: "one",
				})
			require.NoError(err)

			err = network.transferMessage(senderId, receiverId,
				Message{
					Code:    MessageCode_UnitTestProtocol_Ping,
					Payload: "two",
				})
			require.NoError(err)

			err = network.transferMessage(senderId, receiverId,
				Message{
					Code:    MessageCode_UnitTestProtocol_Ping,
					Payload: "three",
				})
			require.NoError(err)
		},
	}

	for name, testFunc := range tests {
		t.Run(name, func(t *testing.T) {

			network := NewNetwork()
			testFunc(t, network)

			network.WaitForDeliveryOfSentMessages()
		})
	}
}

func TestNetwork_NetworkWithLatency_EnforcesDelays(t *testing.T) {
	require := require.New(t)

	senderId := PeerId("sender")
	receiverId := PeerId("receiver")

	ctrl := gomock.NewController(t)
	latencyModel := NewMockLatencyModel(ctrl)

	network := NewNetworkWithLatency(latencyModel)

	_, err := network.NewServer(senderId)
	require.NoError(err)
	receiver, err := network.NewServer(receiverId)
	require.NoError(err)

	msg := Message{
		Code:    MessageCode_UnitTestProtocol_Ping,
		Payload: "ping",
	}

	handler := NewMockMessageHandler(ctrl)
	handler.EXPECT().HandleMessage(senderId, msg)
	receiver.RegisterMessageHandler(handler)

	testDuration := 50 * time.Millisecond
	latencyModel.EXPECT().GetDelay(senderId, receiverId, msg).Return(testDuration)
	start := time.Now()
	err = network.transferMessage(senderId, receiverId, msg)
	require.NoError(err)
	network.WaitForDeliveryOfSentMessages()
	elapsed := time.Since(start)
	require.GreaterOrEqual(elapsed, testDuration)
	require.Less(elapsed, testDuration*2)
}
