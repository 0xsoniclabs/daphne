package node

import (
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/receiptstore"
	"github.com/0xsoniclabs/daphne/daphne/rpc"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// Node is a participant in the P2P network.
//
// A node maintains these essential components:
//   - consensus engine (for active participation or passive observation)
//   - RPC service (for client interactions)
type Node struct {
	consensus consensus.Consensus
	rpc       rpc.Server
}

// newBaseNode creates the common infrastructure shared by all nodes.
// Active nodes additionally get a transaction provider for consensus.
func newBaseNode(
	id p2p.PeerId,
	network *p2p.Network,
) (p2p.Server, rpc.Server, txpool.TxPool, error) {
	if network == nil {
		return nil, nil, nil, fmt.Errorf("cannot create node: network is nil")
	}

	// Create a P2P server instance for the node.
	server, err := network.NewServer(id)
	if err != nil {
		return nil, nil, nil, err
	}

	// Create a transaction pool and install gossip for transaction propagation.
	pool := txpool.NewTxPool()
	txpool.InstallTxGossip(pool, server)

	// Initialize a receipt store for tracking transaction results.
	receipts := receiptstore.NewReceiptStore()
	rpcService := rpc.NewServer(pool, receipts)

	return server, rpcService, pool, nil
}

// NewActiveNode creates a node that participates actively in consensus. Active
// validator nodes run consensus actively, maintain state, and contribute to
// bundle creation and validation.
func NewActiveNode(
	id p2p.PeerId,
	network *p2p.Network,
	factory consensus.Factory,
	genesis state.Genesis,
) (*Node, error) {

	server, rpcService, pool, err := newBaseNode(id, network)
	if err != nil {
		return nil, err
	}

	nodeState := state.New(genesis)
	provider := newTransactionProvider(nodeState, pool)

	consensus := factory.NewActive(server, provider)

	return &Node{
		consensus: consensus,
		rpc:       rpcService,
	}, nil
}

// NewPassiveNode creates a node that participates passively in consensus.
// Passive nodes run consensus in observer mode, maintaining state and following
// decisions of active participants. They also expose an RPC service to clients.
func NewPassiveNode(
	id p2p.PeerId,
	network *p2p.Network,
	factory consensus.Factory,
) (*Node, error) {

	server, rpcService, _, err := newBaseNode(id, network)
	if err != nil {
		return nil, err
	}

	consensus := factory.NewPassive(server)

	return &Node{
		consensus: consensus,
		rpc:       rpcService,
	}, nil
}

// GetRpcService returns the RPC server instance for this node.
func (n *Node) GetRpcService() rpc.Server {
	return n.rpc
}

// Stop gracefully shuts down the node's consensus engine.
func (n *Node) Stop() {
	if n.consensus != nil {
		n.consensus.Stop()
	}
}

// transactionProvider is an adapter to implement consensus.TransactionProvider
// by using state information to determine executable transactions.
type transactionProvider struct {
	state state.State
	pool  txpool.TxPool
}

// newTransactionProvider creates a new adapter that bridges TxPool with
// consensus.TransactionProvider interface.
func newTransactionProvider(
	s state.State,
	p txpool.TxPool,
) *transactionProvider {
	return &transactionProvider{
		state: s,
		pool:  p,
	}
}

// GetNonce returns the latest nonce for the given address from the state.
func (tp *transactionProvider) GetNonce(address types.Address) types.Nonce {
	return tp.state.GetAccount(address).Nonce
}

// GetExecutableTransactions returns executable transactions from the pool
// using this provider as the nonce source.
func (tp *transactionProvider) GetExecutableTransactions() []types.Transaction {
	return tp.pool.GetExecutableTransactions(tp)
}

// GetCandidateTransactions returns candidate transactions for consensus by
// using the underlying txpool's GetExecutableTransactions method with the
// provider itself as the nonce source.
func (tp *transactionProvider) GetCandidateTransactions() []types.Transaction {
	// Access the pool field from the embedded TransactionProvider and call
	// GetExecutableTransactions with the provider as the nonce source
	return tp.GetExecutableTransactions()
}
