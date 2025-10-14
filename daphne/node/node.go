package node

import (
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/receiptstore"
	"github.com/0xsoniclabs/daphne/daphne/rpc"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
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

type NodeConfig struct {
	Network   *p2p.Network
	Broadcast broadcast.Factories
	Consensus consensus.Factory
	Genesis   state.Genesis
	Tracker   tracker.Tracker
}

// newBaseNode creates the common infrastructure shared by all nodes.
// Active nodes additionally get a transaction provider for consensus.
func newBaseNode(
	id p2p.PeerId,
	config NodeConfig,
) (p2p.Server, rpc.Server, txpool.TxPool, state.State, error) {
	if config.Network == nil {
		return nil, nil, nil, nil, fmt.Errorf("cannot create node: network is nil")
	}

	// Augment the tracker with the node ID for context, if a tracker is given.
	tracker := config.Tracker
	if tracker != nil {
		tracker = tracker.With("node", id)
	}

	// Create a P2P server instance for the node.
	server, err := config.Network.NewServer(id)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Create a transaction pool and install gossip for transaction propagation.
	pool := txpool.NewTxPool(tracker)
	txpool.InstallTxPoolSync(pool, server, broadcast.GetFactory[types.Hash, types.Transaction](config.Broadcast))

	// Initialize a receipt store for tracking transaction results.
	receipts := receiptstore.NewReceiptStore()
	rpcService := rpc.NewServer(pool, receipts, tracker)

	// Initialize the chain state.
	state := state.NewStateBuilder().
		WithGenesis(config.Genesis).
		WithTracker(tracker).
		Build()

	return server, rpcService, pool, state, nil
}

// NewActiveNode creates a node that participates actively in consensus. Active
// validator nodes run consensus actively, maintain state, and contribute to
// bundle creation and validation.
func NewActiveNode(
	id p2p.PeerId,
	config NodeConfig,
) (*Node, error) {
	server, rpcService, pool, state, err := newBaseNode(id, config)
	if err != nil {
		return nil, err
	}

	provider := newTransactionProvider(state, pool)

	active := config.Consensus.NewActive(server, provider)

	active.RegisterListener(consensus.WrapBundleListener(func(bundle types.Bundle) {
		state.Apply(bundle.Transactions)
	}))

	return &Node{
		consensus: active,
		rpc:       rpcService,
	}, nil
}

// NewPassiveNode creates a node that participates passively in consensus.
// Passive nodes run consensus in observer mode, maintaining state and following
// decisions of active participants. They also expose an RPC service to clients.
func NewPassiveNode(
	id p2p.PeerId,
	config NodeConfig,
) (*Node, error) {

	server, rpcService, _, state, err := newBaseNode(id, config)
	if err != nil {
		return nil, err
	}

	passive := config.Consensus.NewPassive(server)

	passive.RegisterListener(consensus.WrapBundleListener(func(bundle types.Bundle) {
		state.Apply(bundle.Transactions)
	}))

	return &Node{
		consensus: passive,
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
		n.consensus = nil
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

// GetCandidateTransactions returns candidate transactions for consensus by
// using the underlying txpool's GetExecutableTransactions method with the
// provider itself as the nonce source.
func (tp *transactionProvider) GetCandidateTransactions() []types.Transaction {
	// Access the pool field from the embedded TransactionProvider and call
	// GetExecutableTransactions with the provider as the nonce source
	return tp.pool.GetExecutableTransactions(tp)
}
