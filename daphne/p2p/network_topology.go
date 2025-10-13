package p2p

//go:generate mockgen -source network_topology.go -destination=network_topology_mock.go -package=p2p

// NetworkTopology defines the connectivity graph between peers in the network.
// It determines whether a direct connection should exist between any two peers.
type NetworkTopology interface {
	// ShouldConnect determines if a directed connection should be established
	// from one peer to another.
	ShouldConnect(from, to PeerId) bool
}

// --- FullyMeshedTopology ---

// FullyMeshedTopology implements a topology where every peer is
// connected to every other peer, forming a complete graph.
type FullyMeshedTopology struct{}

// NewFullyMeshedTopology creates a new fully connected topology.
func NewFullyMeshedTopology() *FullyMeshedTopology {
	return &FullyMeshedTopology{}
}

// ShouldConnect in a fully connected topology always returns true, establishing
// a bidirectional link since ShouldConnect(a, b) and ShouldConnect(b, a)
// will both be true for all peers a, b. Peers are not connected to themselves
// in this implementation.
func (t *FullyMeshedTopology) ShouldConnect(from, to PeerId) bool {
	return from != to
}
