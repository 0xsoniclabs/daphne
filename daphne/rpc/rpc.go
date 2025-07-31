package rpc

import (
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

//go:generate mockgen -source rpc.go -destination=rpc_mock.go -package=rpc

var (
	ErrNotFound = fmt.Errorf("not found")
)

// Serve provides an end-user interface for customers to interact with the chain.
type Server interface {
	// Send requests the given transaction to be queued for execution on the
	// chain. Send will fail if the the receiving node determines that it can
	// not be processed. For instance, if the used nonce has been used before.
	Send(tx types.Transaction) error

	// IsPending checks whether the local node knows of the transaction as not
	// yet being included in a block.
	IsPending(hash types.Hash) bool

	// GetReceipt retrieves the receipt for a transaction by its hash. If the
	// transaction is not found, ErrNotFound will be returned.
	GetReceipt(hash types.Hash) (types.Receipt, error)
}

type Backend interface {
	Add(tx types.Transaction) error
	Contains(hash types.Hash) bool
	GetReceipt(hash types.Hash) (types.Receipt, bool)
}

// NewServer creates a new RPC server that allows users to interact with a node.
func NewServer(backend Backend) *server {
	return &server{
		backend: backend,
	}
}

type server struct {
	backend Backend
}

func (s *server) Send(tx types.Transaction) error {
	return s.backend.Add(tx)
}

func (s *server) IsPending(hash types.Hash) bool {
	return s.backend.Contains(hash)
}

func (s *server) GetReceipt(hash types.Hash) (types.Receipt, error) {
	if receipt, found := s.backend.GetReceipt(hash); found {
		return receipt, nil
	}
	return types.Receipt{}, ErrNotFound
}
