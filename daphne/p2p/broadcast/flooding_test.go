package broadcast

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

// TestFlooding covers all generic channel tests.
func TestFlooding_ChannelProperties(t *testing.T) {
	testChannelImplementation(t, NewFlooding[string, int])
}

func TestFlooding_Broadcast_EmitsMessageListingAllKnownPeers(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())

	channel := NewFlooding(server, intToString)

	localId := p2p.PeerId("local")
	peers := []p2p.PeerId{"peer1", "peer2", "peer3"}
	server.EXPECT().GetPeers().Return(peers)
	server.EXPECT().GetLocalId().Return(localId)

	// The message that should be send to all peers, listing all peers
	// the message is going to be sent to plus the local peer who already
	// knows about the message.
	msg := FloodingMessage[int]{
		Payload:  12,
		Notified: sets.New(append([]p2p.PeerId{localId}, peers...)...),
	}

	for _, peer := range peers {
		server.EXPECT().SendMessage(peer, msg)
	}

	channel.Broadcast(12)
}

func TestFlooding_handleMessage_ForwardNewMessageWithExtendedPeerSet(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())

	localId := p2p.PeerId("local")
	server.EXPECT().GetLocalId().Return(localId).AnyTimes()

	notifiedPeers := sets.New[p2p.PeerId]("peer1", "peer2")
	localPeers := sets.New[p2p.PeerId]("peer2", "peer3")

	incoming := FloodingMessage[int]{
		Payload:  42,
		Notified: notifiedPeers,
	}

	outgoing := FloodingMessage[int]{
		Payload:  42,
		Notified: sets.Union(sets.New(localId), notifiedPeers, localPeers),
	}

	channel := newFlooding(server, intToString)
	require.NotNil(channel)

	server.EXPECT().GetPeers().Return(localPeers.ToSlice())
	server.EXPECT().SendMessage(p2p.PeerId("peer3"), outgoing)

	channel.handleMessage("peer1", incoming)
}

func TestFlooding_handleMessage_IgnoresNonFloodingMessages(t *testing.T) {
	channel := &flooding[int, int]{}
	channel.handleMessage("peer1", "not a flooding message")
}

func TestFlooding_handleMessage_AddsUnknownMessagesToSeenSet(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()
	server.EXPECT().RegisterMessageHandler(gomock.Any())
	server.EXPECT().GetPeers().Return(nil)

	channel := newFlooding(server, intToString)

	require.False(channel.seen.Contains("1"))
	channel.handleMessage("peer1", FloodingMessage[int]{Payload: 1})
	require.True(channel.seen.Contains("1"))
}

func TestFlooding_handleMessage_IgnoresSeenMessages(t *testing.T) {
	channel := &flooding[string, int]{
		extractKeyFromMessage: intToString,
	}
	channel.seen.Add("1")
	// If the seen message would not be ignored, a message would be sent,
	// which would fail the test since no server is present (nil-pointer deref).
	channel.handleMessage("peer1", FloodingMessage[int]{Payload: 1})
}

func TestFlooding_sendMessage_ForwardsMessageToAllPeersOnce(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)

	channel := &flooding[string, int]{
		p2pServer: server,
	}

	peer1 := p2p.PeerId("peer1")
	peer2 := p2p.PeerId("peer2")
	peer3 := p2p.PeerId("peer3")
	peers := sets.New(peer1, peer2, peer3)

	msg := FloodingMessage[int]{
		Payload:  100,
		Notified: sets.New(peer1, peer2, peer3),
	}

	server.EXPECT().SendMessage(peer1, gomock.Any())
	server.EXPECT().SendMessage(peer2, gomock.Any())
	server.EXPECT().SendMessage(peer3, gomock.Any())
	channel.sendMessages(peers, msg)
}

func TestFlooding_sendMessage_RemovesFailedPeersFromNotifiedSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().GetLocalId().Return(p2p.PeerId("self")).AnyTimes()

	channel := &flooding[string, int]{
		p2pServer: server,
	}

	peer1 := p2p.PeerId("peer1")
	peer2 := p2p.PeerId("peer2")
	peer3 := p2p.PeerId("peer3")
	peer4 := p2p.PeerId("peer4")
	peers := sets.New(peer1, peer2, peer3, peer4)

	msg := FloodingMessage[int]{
		Payload:  100,
		Notified: peers.Clone(),
	}

	// Messages are delivered in random order to peers, so we need to
	// handle to handle an arbitrary order of SendMessage calls. The test
	// simulates send failures for peer2 and peer3, which should be removed
	// from the Notified set in the message.
	nextMsgExpected := FloodingMessage[int]{
		Payload:  100,
		Notified: peers.Clone(),
	}
	send := func(peer p2p.PeerId, message p2p.Message) error {
		require.Equal(t, nextMsgExpected, message)
		if peer == peer2 || peer == peer3 {
			// Simulate send failure for peer2 and peer3
			nextMsgExpected.Notified.Remove(peer)
			return fmt.Errorf("injected issue")
		}
		return nil
	}

	server.EXPECT().SendMessage(peer1, gomock.Any()).DoAndReturn(send)
	server.EXPECT().SendMessage(peer2, gomock.Any()).DoAndReturn(send)
	server.EXPECT().SendMessage(peer3, gomock.Any()).DoAndReturn(send)
	server.EXPECT().SendMessage(peer4, gomock.Any()).DoAndReturn(send)

	channel.sendMessages(peers, msg)

	require.Equal(t, sets.New(peer1, peer4), msg.Notified)
}

func TestFloodingMessage_ReportsReadableMessageType(t *testing.T) {
	msg := FloodingMessage[types.Transaction]{}
	require.EqualValues(t, "FloodingMessage[Transaction]", p2p.GetMessageType(msg))
}
