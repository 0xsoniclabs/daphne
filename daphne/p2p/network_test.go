package p2p

import (
	"sync/atomic"
	"testing"
	"testing/synctest"
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

	synctest.Test(t, func(t *testing.T) {
		require.NoError(t, server1.SendMessage(id2, msg))
		synctest.Wait()
	})
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

	synctest.Test(t, func(t *testing.T) {
		require.NoError(network.transferMessage(A, B, msg))
		require.NoError(network.transferMessage(B, A, msg))
		synctest.Wait()
	})
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
			synctest.Test(t, func(t *testing.T) {
				testFunc(t, network)
				synctest.Wait()
			})
		})
	}
}

func TestNetwork_NewNetworkWithLatency_EnforcesDelays(t *testing.T) {
	tests := map[string]struct {
		sendDelay     time.Duration
		deliveryDelay time.Duration
	}{
		"send delay only": {
			sendDelay:     60 * time.Millisecond,
			deliveryDelay: 0 * time.Millisecond,
		},
		"delivery delay only": {
			sendDelay:     0 * time.Millisecond,
			deliveryDelay: 50 * time.Millisecond,
		},
		"send and delivery delay": {
			sendDelay:     60 * time.Millisecond,
			deliveryDelay: 50 * time.Millisecond,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				require := require.New(t)

				senderId := PeerId("sender")
				receiverId := PeerId("receiver")

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				latencyModel := NewMockLatencyModel(ctrl)

				network := NewNetworkBuilder().WithLatency(latencyModel).Build()

				_, err := network.NewServer(senderId)
				require.NoError(err)
				receiver, err := network.NewServer(receiverId)
				require.NoError(err)

				msg := Message{
					Code:    MessageCode_UnitTestProtocol_Ping,
					Payload: "ping",
				}

				var receiveTime atomic.Value
				handler := NewMockMessageHandler(ctrl)
				handler.EXPECT().HandleMessage(senderId, msg).Do(func(PeerId, Message) {
					receiveTime.Store(time.Now())
				})
				receiver.RegisterMessageHandler(handler)

				latencyModel.EXPECT().GetSendDelay(
					senderId,
					receiverId,
					msg,
				).Return(testCase.sendDelay)
				latencyModel.EXPECT().GetDeliveryDelay(
					senderId,
					receiverId,
					msg,
				).Return(testCase.deliveryDelay)

				start := time.Now()
				err = network.transferMessage(senderId, receiverId, msg)
				require.NoError(err)

				time.Sleep(testCase.sendDelay + testCase.deliveryDelay +
					5*time.Millisecond)

				elapsed := receiveTime.Load().(time.Time).Sub(start)
				require.Equal(testCase.sendDelay+testCase.deliveryDelay, elapsed)
			})
		})
	}
}

func TestNetwork_NewNetworkWithTopology_AppliesTopology(t *testing.T) {
	require := require.New(t)
	id1 := PeerId("server1")
	id2 := PeerId("server2")
	id3 := PeerId("server3")

	ctrl := gomock.NewController(t)
	topology := NewMockNetworkTopology(ctrl)

	// Define the expected connections based on the topology.
	// 1 <-> 2
	topology.EXPECT().ShouldConnect(id1, id2).Return(true)
	topology.EXPECT().ShouldConnect(id2, id1).Return(true)
	// 1 <-/-> 3
	topology.EXPECT().ShouldConnect(id1, id3).Return(false)
	topology.EXPECT().ShouldConnect(id3, id1).Return(false)
	// 2 -> 3
	topology.EXPECT().ShouldConnect(id2, id3).Return(true)
	topology.EXPECT().ShouldConnect(id3, id2).Return(false)

	network := NewNetworkBuilder().WithTopology(topology).Build()
	server1, err := network.NewServer(id1)
	require.NoError(err)
	server2, err := network.NewServer(id2)
	require.NoError(err)
	server3, err := network.NewServer(id3)
	require.NoError(err)

	require.ElementsMatch([]PeerId{id2}, server1.GetPeers())
	require.ElementsMatch([]PeerId{id1, id3}, server2.GetPeers())
	require.ElementsMatch([]PeerId{}, server3.GetPeers())
}
