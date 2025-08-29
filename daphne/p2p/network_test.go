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

func TestNetwork_SetBaseDelay_ConfiguresDelayCorrectly(t *testing.T) {
	tests := map[string]struct {
		delay time.Duration
	}{
		"sets 200ms delay correctly": {
			delay: 200 * time.Millisecond,
		},
		"sets 50ms delay correctly": {
			delay: 50 * time.Millisecond,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)

			latencyModel := NewFixedDelayModel()
			latencyModel.SetBaseDelay(testCase.delay)
			network := NewNetworkWithLatency(latencyModel)

			senderId := PeerId("sender")
			receiverId := PeerId("receiver")

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

			now := time.Now()
			err = network.transferMessage(senderId, receiverId, msg)
			require.NoError(err)
			network.WaitForDeliveryOfSentMessages()
			require.GreaterOrEqual(time.Since(now), testCase.delay)
			require.Less(time.Since(now), testCase.delay+50*time.Millisecond)
		})
	}
}

func TestNetwork_SetConnectionDelay_ConfiguresDelayCorrectly(t *testing.T) {
	tests := map[string]struct {
		delay time.Duration
	}{
		"sets 200ms delay correctly": {
			delay: 200 * time.Millisecond,
		},
		"sets 50ms delay correctly": {
			delay: 50 * time.Millisecond,
		},
	}
	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)

			// Test delayed connection
			senderId := PeerId("sender")
			receiverId := PeerId("receiver")
			latencyModel := NewFixedDelayModel()
			latencyModel.SetConnectionDelay(senderId, receiverId, testCase.delay)
			network := NewNetworkWithLatency(latencyModel)

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

			now := time.Now()
			err = network.transferMessage(senderId, receiverId, msg)
			require.NoError(err)
			network.WaitForDeliveryOfSentMessages()
			require.GreaterOrEqual(time.Since(now), testCase.delay)
			require.Less(time.Since(now), testCase.delay+100*time.Millisecond)

			// Test vanilla connection (no delay)
			vanillaSenderId := PeerId("vanilla_sender")
			vanillaReceiverId := PeerId("vanilla_receiver")
			_, err = network.NewServer(vanillaSenderId)
			require.NoError(err)
			vanillaReceiver, err := network.NewServer(vanillaReceiverId)
			require.NoError(err)

			vanillaHandler := NewMockMessageHandler(ctrl)
			vanillaHandler.EXPECT().HandleMessage(vanillaSenderId, msg)
			vanillaReceiver.RegisterMessageHandler(vanillaHandler)

			vanillaNow := time.Now()
			err = network.transferMessage(vanillaSenderId, vanillaReceiverId, msg)
			require.NoError(err)
			network.WaitForDeliveryOfSentMessages()
			require.Less(time.Since(vanillaNow), 50*time.Millisecond)
		})
	}
}
