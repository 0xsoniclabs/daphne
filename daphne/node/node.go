package node

import (
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/rpc"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
)

// Node represents a participant in the P2P network that hosts an RPC service.
type Node struct {
	id  p2p.PeerId
	rpc rpc.Server
}

// NewRpcNode creates a new RPC node within provided P2P network.
func NewRpcNode(id p2p.PeerId, network *p2p.Network) (*Node, error) {
	return newNode(id, network)
}

func newNode(id p2p.PeerId, network *p2p.Network) (*Node, error) {
	if network == nil {
		return nil, fmt.Errorf("cannot create node: network is nil")
	}

	p2p, err := network.NewServer(id)
	if err != nil {
		return nil, err
	}

	pool := txpool.NewTxPool()
	txpool.InstallTxGossip(pool, p2p)

	return &Node{
		id:  id,
		rpc: rpc.NewServer(pool),
	}, nil
}

func (n *Node) GetRpcService() rpc.Server {
	return n.rpc
}
