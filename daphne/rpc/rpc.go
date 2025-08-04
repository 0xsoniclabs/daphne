package rpc

import (
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// Server provides an end-user interface for customers to interact with the chain.
type Server interface {
	// Send requests the given transaction to be queued for execution on the
	// chain. Send will fail if the the receiving node determines that it can
	// not be processed. For instance, if the used nonce has been used before.
	Send(tx types.Transaction) error

	// IsPending checks whether the local node knows of the transaction as not
	// yet being included in a block.
	IsPending(hash types.Hash) bool
}

// NewServer creates a new RPC server that allows users to interact with a node.
func NewServer(pool txpool.TxPool) *server {
	return &server{
		pool: pool,
	}
}

type server struct {
	pool txpool.TxPool
}

func (s *server) Send(tx types.Transaction) error {
	return s.pool.Add(tx)
}

func (s *server) IsPending(hash types.Hash) bool {
	return s.pool.Contains(hash)
}
