// Copyright 2026 Sonic Operations Ltd
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

	msg := "ping"

	err = server1.SendMessage(id2, msg)
	require.Error(err, "Expected error when sending to non-connected peer")
	require.EqualError(err, "cannot send message to peer server2: not connected")
}

func TestServer_receiveMessage_DeliversCallbacksAsynchronously(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		id := PeerId("sender")
		server := &server{}

		msg := "ping"

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

func TestWrapMessageHandler_CanBeUsedAsAMessageHandler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		sender := PeerId("sender")
		msg := 12

		handler := NewMockMessageHandler(ctrl)
		handler.EXPECT().HandleMessage(sender, msg)

		server := &server{}
		server.RegisterMessageHandler(WrapMessageHandler(handler.HandleMessage))

		server.receiveMessage(sender, msg)

		synctest.Wait()
	})
}
