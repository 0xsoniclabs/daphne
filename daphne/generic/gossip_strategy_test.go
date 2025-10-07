package generic

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/stretchr/testify/require"
)

// --- FloodFallbackStrategy Tests ---

func TestGossipStrategy_FloodFallbackStrategyShouldGossip_ReturnsTrueWhenPeerUnknown(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.True(t, shouldGossip, "Should gossip to unknown peer")
}

func TestGossipStrategy_FloodFallbackStrategyShouldGossip_ReturnsTrueWhenMessageUnknownToPeer(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	// Mark a different message as known
	strategy.OnSent(p2p.PeerId("peer1"), "msg1")
	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg2")

	require.True(t, shouldGossip, "Should gossip unknown message to known peer")
}

func TestGossipStrategy_FloodFallbackStrategyShouldGossip_ReturnsFalseAfterOnReceived(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	strategy.OnReceived(p2p.PeerId("peer1"), "msg1")
	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.False(t, shouldGossip, "Should not gossip after receiving from peer")
}

func TestGossipStrategy_FloodFallbackStrategyShouldGossip_ReturnsFalseAfterOnSent(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	strategy.OnSent(p2p.PeerId("peer1"), "msg1")
	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.False(t, shouldGossip, "Should not gossip after sending to peer")
}

func TestGossipStrategy_FloodFallbackStrategyShouldGossip_ReturnsFalseAfterOnSendFailed(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	strategy.OnSendFailed(p2p.PeerId("peer1"), "msg1", fmt.Errorf("send error"))
	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.False(t, shouldGossip, "Should not retry after send failure")
}

func TestGossipStrategy_FloodFallbackStrategyOnReceived_MarksMessageAsKnown(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	strategy.OnReceived(p2p.PeerId("peer1"), "msg1")
	// Verify by checking ShouldGossip returns false
	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.False(t, shouldGossip, "OnReceived should mark message as known")
}

func TestGossipStrategy_FloodFallbackStrategyOnSent_MarksMessageAsKnown(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	strategy.OnSent(p2p.PeerId("peer1"), "msg1")
	// Verify by checking ShouldGossip returns false
	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.False(t, shouldGossip, "OnSent should mark message as known")
}

func TestGossipStrategy_FloodFallbackStrategyOnSendFailed_MarksMessageAsKnown(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	strategy.OnSendFailed(p2p.PeerId("peer1"), "msg1", fmt.Errorf("network error"))
	// Verify by checking ShouldGossip returns false
	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.False(t, shouldGossip, "OnSendFailed should mark message as known to prevent retry loops")
}

func TestGossipStrategy_FloodFallbackStrategy_TracksMultiplePeers(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	strategy.OnSent(p2p.PeerId("peer1"), "msg1")
	strategy.OnReceived(p2p.PeerId("peer2"), "msg1")

	shouldGossipPeer1 := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")
	shouldGossipPeer2 := strategy.ShouldGossip(p2p.PeerId("peer2"), "msg1")
	shouldGossipPeer3 := strategy.ShouldGossip(p2p.PeerId("peer3"), "msg1")

	require.False(t, shouldGossipPeer1, "Peer1 should have message marked as known")
	require.False(t, shouldGossipPeer2, "Peer2 should have message marked as known")
	require.True(t, shouldGossipPeer3, "Peer3 should not have message marked as known")
}

func TestGossipStrategy_FloodFallbackStrategy_TracksMultipleMessages(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	strategy.OnSent(p2p.PeerId("peer1"), "msg1")
	strategy.OnSent(p2p.PeerId("peer1"), "msg2")

	shouldGossipMsg1 := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")
	shouldGossipMsg2 := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg2")
	shouldGossipMsg3 := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg3")

	require.False(t, shouldGossipMsg1, "Message 1 should be marked as known")
	require.False(t, shouldGossipMsg2, "Message 2 should be marked as known")
	require.True(t, shouldGossipMsg3, "Message 3 should not be marked as known")
}

func TestGossipStrategy_FloodFallbackStrategy_IsThreadSafe(t *testing.T) {
	strategy := NewFloodFallbackStrategy[string]()

	done := make(chan bool, 4)
	go func() {
		strategy.OnReceived(p2p.PeerId("peer1"), "msg1")
		done <- true
	}()
	go func() {
		strategy.OnSent(p2p.PeerId("peer2"), "msg1")
		done <- true
	}()
	go func() {
		strategy.ShouldGossip(p2p.PeerId("peer3"), "msg1")
		done <- true
	}()
	go func() {
		strategy.OnSendFailed(p2p.PeerId("peer4"), "msg1", fmt.Errorf("error"))
		done <- true
	}()

	// Wait for all goroutines
	for range 4 {
		<-done
	}
}

// --- NoGossipStrategy Tests ---

func TestGossipStrategy_NoGossipStrategyShouldGossip_AlwaysReturnsFalse(t *testing.T) {
	strategy := NewNoGossipStrategy[string]()

	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.False(t, shouldGossip, "NoGossipStrategy should never gossip")
}

func TestGossipStrategy_NoGossipStrategyShouldGossip_ReturnsFalseForMultiplePeers(t *testing.T) {
	strategy := NewNoGossipStrategy[string]()

	shouldGossipPeer1 := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")
	shouldGossipPeer2 := strategy.ShouldGossip(p2p.PeerId("peer2"), "msg1")
	shouldGossipPeer3 := strategy.ShouldGossip(p2p.PeerId("peer3"), "msg2")

	require.False(t, shouldGossipPeer1, "NoGossipStrategy should never gossip to peer1")
	require.False(t, shouldGossipPeer2, "NoGossipStrategy should never gossip to peer2")
	require.False(t, shouldGossipPeer3, "NoGossipStrategy should never gossip to peer3")
}

func TestGossipStrategy_NoGossipStrategyOnReceived_DoesNothing(t *testing.T) {
	strategy := NewNoGossipStrategy[string]()

	// Should not panic
	strategy.OnReceived(p2p.PeerId("peer1"), "msg1")
	// Still should not gossip
	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.False(t, shouldGossip, "NoGossipStrategy should still not gossip after OnReceived")
}

func TestGossipStrategy_NoGossipStrategyOnSent_DoesNothing(t *testing.T) {
	strategy := NewNoGossipStrategy[string]()

	// Should not panic
	strategy.OnSent(p2p.PeerId("peer1"), "msg1")
	// Still should not gossip
	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.False(t, shouldGossip, "NoGossipStrategy should still not gossip after OnSent")
}

func TestGossipStrategy_NoGossipStrategyOnSendFailed_DoesNothing(t *testing.T) {
	strategy := NewNoGossipStrategy[string]()

	// Should not panic
	strategy.OnSendFailed(p2p.PeerId("peer1"), "msg1", fmt.Errorf("error"))
	// Still should not gossip
	shouldGossip := strategy.ShouldGossip(p2p.PeerId("peer1"), "msg1")

	require.False(t, shouldGossip, "NoGossipStrategy should still not gossip after OnSendFailed")
}
