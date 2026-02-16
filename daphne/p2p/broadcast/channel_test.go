// Copyright 2026 Sonic Labs
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package broadcast

import (
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestReceiver_WrapReceiver_CallsWrappedFunction(t *testing.T) {
	called := false
	receiver := WrapReceiver(func(message int) {
		require.Equal(t, 42, message)
		called = true
	})

	receiver.OnMessage(42)
	require.True(t, called, "wrapped function was not called")
}

func TestReceivers_Deliver_DeliversCallbackToRegisteredReceivers(t *testing.T) {
	ctrl := gomock.NewController(t)
	receiverA := NewMockReceiver[int](ctrl)
	receiverB := NewMockReceiver[int](ctrl)

	receivers := Receivers[int]{}
	receivers.Register(receiverA)
	receivers.Register(receiverB)

	receiverA.EXPECT().OnMessage(42)
	receiverB.EXPECT().OnMessage(42)

	// Delivery runs asynchronously, so we need to wait for it to complete. The
	// synctest package provides a simple way to do this.
	synctest.Test(t, func(t *testing.T) {
		receivers.Deliver(42)
	})
}

func TestReceivers_Register_AddsReceiverToRegistry(t *testing.T) {
	ctrl := gomock.NewController(t)
	receiver := NewMockReceiver[int](ctrl)

	receivers := Receivers[int]{}
	require.Empty(t, receivers.registrations)

	receivers.Register(receiver)
	require.Len(t, receivers.registrations, 1)
	require.Equal(t, receiver, receivers.registrations[0].receiver)
}

func TestReceivers_Register_MultipleRegistrationsAreIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	receiver := NewMockReceiver[int](ctrl)

	receivers := Receivers[int]{}
	require.Empty(t, receivers.registrations)

	receivers.Register(receiver)
	require.Len(t, receivers.registrations, 1)
	require.Equal(t, receiver, receivers.registrations[0].receiver)

	receivers.Register(receiver)
	require.Len(t, receivers.registrations, 1)
	require.Equal(t, receiver, receivers.registrations[0].receiver)
}

func TestReceivers_Unregister_RemovesReceiverFromRegistry(t *testing.T) {
	ctrl := gomock.NewController(t)
	receiverA := NewMockReceiver[int](ctrl)
	receiverB := NewMockReceiver[int](ctrl)

	receivers := Receivers[int]{}
	receivers.Register(receiverA)
	receivers.Register(receiverB)
	require.Len(t, receivers.registrations, 2)

	receivers.Unregister(receiverA)
	require.Len(t, receivers.registrations, 1)
	require.Equal(t, receiverB, receivers.registrations[0].receiver)

	receivers.Unregister(receiverB)
	require.Empty(t, receivers.registrations)
}

func TestReceivers_Unregister_BlocksUntilOngoingDeliveriesComplete(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		receiver := NewMockReceiver[int](ctrl)

		receivers := Receivers[int]{}
		receivers.Register(receiver)

		// Set up the receiver to block when receiving a message.
		quit := make(chan struct{})
		receiver.EXPECT().OnMessage(42).Do(func(int) {
			<-quit
		})

		// Start delivery in a separate goroutine.
		go func() {
			receivers.Deliver(42)
		}()

		// Give some time for the delivery to start.
		synctest.Wait()

		// Start unregistering the receiver in a separate goroutine.
		unregisterDone := make(chan struct{})
		go func() {
			receivers.Unregister(receiver)
			close(unregisterDone)
		}()

		// Give some time for Unregister to be called.
		synctest.Wait()

		// At this point, Unregister should be blocked waiting for OnMessage to complete.
		select {
		case <-unregisterDone:
			t.Fatal("Unregister returned before OnMessage completed")
		default:
			// Expected case.
		}

		// Now unblock the receiver.
		close(quit)

		// Wait for OnMessage to complete.
		synctest.Wait()

		// Now Unregister should complete.
		select {
		case <-unregisterDone:
			// Expected case.
		default:
			t.Fatal("Unregister did not return after OnMessage completed")
		}
	})
}

// --- Generic Channel Tests ---

// testChannelImplementation runs a suite of generic tests for a Channel
// implementation. It is used to verify that different Channel implementations
// (e.g., Gossip, Flooding) behave correctly and consistently.
func testChannelImplementation(
	t *testing.T,
	factory Factory[string, int],
) {
	t.Helper()

	t.Run("broadcast_notifies_local_receivers", func(t *testing.T) {
		testGossip_Broadcast_NotifiesLocalReceivers(t, factory)
	})

	t.Run("broadcast_delivers_callbacks_asynchronously", func(t *testing.T) {
		testChannel_Broadcast_DeliversCallbacksAsynchronously(t, factory)
	})

	t.Run("register_adds_receivers_to_list", func(t *testing.T) {
		testChannel_Register_AddsReceiversToList(t, factory)
	})

	t.Run("unregister_removes_receiver_from_list", func(t *testing.T) {
		testChannel_Unregister_RemovesReceiverFromList(t, factory)
	})

	t.Run("integration_test/delivers_messages_to_all_participants_exactly_once", func(t *testing.T) {
		testChannel_IntegrationTest_DeliversMessagesToAllParticipantsExactlyOnce(t, factory)
	})

	t.Run("integration_test/works_with_real_p2p_server", func(t *testing.T) {
		testGossip_IntegrationTest_WorksWithRealP2pServer(t, factory)
	})
}

func testGossip_Broadcast_NotifiesLocalReceivers(
	t *testing.T,
	factory Factory[string, int],
) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		server.EXPECT().GetLocalId().Return(p2p.PeerId("sender")).AnyTimes()
		server.EXPECT().GetPeers().Return([]p2p.PeerId{}).AnyTimes()

		channel := factory(server, intToString)

		receiver := NewMockReceiver[int](ctrl)
		receiver.EXPECT().OnMessage(1)

		channel.Register(receiver)
		channel.Broadcast(1)
		synctest.Wait()
	})
}

func testChannel_Broadcast_DeliversCallbacksAsynchronously(
	t *testing.T,
	factory Factory[string, int],
) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
	server.EXPECT().GetLocalId().Return(p2p.PeerId("sender")).AnyTimes()
	server.EXPECT().GetPeers().Return([]p2p.PeerId{}).AnyTimes()

	channel := factory(server, intToString)

	quit := make(chan struct{})
	done := make(chan struct{})
	receiver := NewMockReceiver[int](ctrl)
	receiver.EXPECT().OnMessage(1).Do(func(int) {
		<-quit
		close(done)
	})

	channel.Register(receiver)
	channel.Broadcast(1)
	close(quit)
	<-done
}

func testChannel_Register_AddsReceiversToList(
	t *testing.T,
	factory Factory[string, int],
) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		p2pServer := p2p.NewMockServer(ctrl)
		// This method is irrelevant for the test.
		p2pServer.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
		p2pServer.EXPECT().GetPeers().Return(nil).AnyTimes()
		p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

		channel := factory(p2pServer, intToString)

		receiver := NewMockReceiver[int](ctrl)
		receiver.EXPECT().OnMessage(1)

		channel.Register(receiver)
		channel.Register(receiver) // Duplicate registration should be ignored.
		channel.Broadcast(1)
		synctest.Wait()
	})
}

func testChannel_Unregister_RemovesReceiverFromList(
	t *testing.T,
	factory Factory[string, int],
) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		p2pServer := p2p.NewMockServer(ctrl)
		// These methods are irrelevant for the test.
		p2pServer.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		p2pServer.EXPECT().GetLocalId().AnyTimes()
		p2pServer.EXPECT().GetPeers().AnyTimes()

		channel := factory(p2pServer, intToString)

		receivers := make([]*MockReceiver[int], 0, 3)
		for i := range 3 {
			receiver := NewMockReceiver[int](ctrl)
			if i == 1 {
				receiver.EXPECT().OnMessage(1).Times(0)
			} else {
				receiver.EXPECT().OnMessage(1)
			}
			channel.Register(receiver)
			receivers = append(receivers, receiver)
		}

		// Unregister the second receiver.
		channel.Unregister(receivers[1])
		channel.Broadcast(1)
		synctest.Wait()
	})
}

func testChannel_IntegrationTest_DeliversMessagesToAllParticipantsExactlyOnce(
	t *testing.T,
	factory Factory[string, int],
) {
	const NumNodes = 5
	const NumMessages = NumNodes * NumNodes * 2
	require := require.New(t)
	ctrl := gomock.NewController(t)

	// Creates a network of N peers.
	net := p2p.NewNetwork()
	peers := []p2p.Server{}
	for i := range NumNodes {
		id := p2p.PeerId(fmt.Sprintf("peer-%d", i))
		peer, err := net.NewServer(id)
		require.NoError(err)
		peers = append(peers, peer)
	}

	// Install flooding broadcaster on each peer.
	channels := []Channel[int]{}
	for i := range peers {
		channel := factory(peers[i], intToString)
		channels = append(channels, channel)
	}

	// Install listeners on each channel to verify incoming messages.
	for i := range channels {
		listener := NewMockReceiver[int](ctrl)
		// Each listener should receive all messages.
		for j := range NumMessages {
			listener.EXPECT().OnMessage(j)
		}
		channels[i].Register(listener)
	}

	synctest.Test(t, func(t *testing.T) {
		// Broadcast M messages from various channels.
		for j := range NumMessages {
			channelId := j % NumNodes
			channels[channelId].Broadcast(j)
		}
	})
}

func testGossip_IntegrationTest_WorksWithRealP2pServer(
	t *testing.T,
	factory Factory[string, int],
) {
	synctest.Test(t, func(t *testing.T) {
		// This test checks if the a channel works correctly with the P2P server.

		network := p2p.NewNetwork()
		servers := make([]p2p.Server, 5)
		for i := range 5 {
			pid := p2p.PeerId(fmt.Sprintf("peer-%d", i+1))
			server, err := network.NewServer(pid)
			require.NoError(t, err, "Failed to create server %d", i+1)
			servers[i] = server
		}

		ctrl := gomock.NewController(t)

		channels := make([]Channel[int], 5)
		for i, server := range servers {
			channel := factory(server, intToString)

			receiver := NewMockReceiver[int](ctrl)
			for j := range servers {
				receiver.EXPECT().OnMessage(j)
			}

			channel.Register(receiver)
			channels[i] = channel
		}

		for i := range 5 {
			channels[i].Broadcast(i)
		}

		synctest.Wait()
	})
}

func intToString(i int) string {
	return fmt.Sprintf("%d", i)
}
