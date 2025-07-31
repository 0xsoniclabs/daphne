package node

import (
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/rpc"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
)

type Node struct {
	id     p2p.PeerId
	txPool txpool.TxPool
	rpc    rpc.Server
	p2p    p2p.Server
}

func NewNode(id p2p.PeerId, network *p2p.Network) (*Node, error) {

	p2p, err := network.NewServer(id)
	if err != nil {
		return nil, err
	}

	pool := txpool.NewTxPool()

	// Install protocols.
	txpool.NewTxGossip(pool, p2p)

	return &Node{
		id:     id,
		p2p:    p2p,
		rpc:    rpc.NewServer(pool),
		txPool: pool,
	}, nil
}

func (n *Node) GetRpcService() rpc.Server {
	return n.rpc
}
