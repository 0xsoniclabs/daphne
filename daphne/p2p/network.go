package p2p

// Network is a P2P network that manages the inter-connection of peers and
// forwards messages between those peers.
type Network struct {
	peers map[PeerId]*server
}

// NewNetwork creates a new, empty P2P network.
func NewNetwork() *Network {
	return &Network{
		peers: make(map[PeerId]*server),
	}
}
