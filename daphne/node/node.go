package node

import (
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/receipt_store"
	"github.com/0xsoniclabs/daphne/daphne/rpc"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
)

// Node is a participant in the P2P network that hosts an RPC service.
type Node struct {
	id  p2p.PeerId
	rpc rpc.Server
}

// New creates a Node with the given PeerId and connects it to the provided network.
func New(id p2p.PeerId, network *p2p.Network) (*Node, error) {
	if network == nil {
		return nil, fmt.Errorf("cannot create node: network is nil")
	}

	server, err := network.NewServer(id)
	if err != nil {
		return nil, err
	}

	pool := txpool.NewTxPool()
	txpool.InstallTxGossip(pool, server)

	return &Node{
		id:  id,
		rpc: rpc.NewServer(pool, receipt_store.NewReceiptStore()),
	}, nil
}

func (n *Node) GetRpcService() rpc.Server {
	return n.rpc
}
