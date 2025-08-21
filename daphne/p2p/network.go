package p2p

import (
	"fmt"
)

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

// NewServer creates a new server on this P2P network with the given PeerId. The
// resulting server instance can be used by a node to interact with the network.
func (n *Network) NewServer(id PeerId) (Server, error) {
	res, err := NewServer(id, n)
	if err != nil {
		return nil, fmt.Errorf("failed to create server with ID %s: %w", id, err)
	}
	n.peers[id] = res
	return res, nil
}
