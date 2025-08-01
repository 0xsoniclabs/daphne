package node

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/rpc"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/store"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

type Node struct {
	id         p2p.PeerId
	chainState state.State
	txPool     txpool.TxPool
	rpc        rpc.Server
	p2p        p2p.Server
	store      *store.Store
}

func NewValidator(
	id p2p.PeerId,
	genesis map[types.Address]state.Account,
	network *p2p.Network,
	consensus consensus.Algorithm,
) (*Node, error) {
	return newNode(id, genesis, network, consensus, true)
}

func NewRpc(
	id p2p.PeerId,
	genesis map[types.Address]state.Account,
	network *p2p.Network,
	consensus consensus.Algorithm,
) (*Node, error) {
	return newNode(id, genesis, network, consensus, false)
}

func newNode(
	id p2p.PeerId,
	genesis map[types.Address]state.Account,
	network *p2p.Network,
	algorithm consensus.Algorithm,
	isValidator bool,
) (*Node, error) {

	p2p, err := network.NewServer(id)
	if err != nil {
		return nil, err
	}

	state := state.New(genesis)

	pool := txpool.NewTxPool()

	// Install protocols.
	txpool.NewTxGossip(pool, p2p)

	store := &store.Store{}

	var consensus consensus.Consensus
	var rpcServer rpc.Server
	if isValidator {
		consensus = algorithm.NewActive(p2p, &payloadSourceAdapter{
			state: state,
			pool:  pool,
		})
	} else {
		consensus = algorithm.NewPassive(p2p)
		rpcServer = rpc.NewServer(rpcBackendAdapter{
			TxPool: pool,
			Store:  store,
		})
	}

	res := &Node{
		id:         id,
		chainState: state,
		txPool:     pool,
		p2p:        p2p,
		rpc:        rpcServer,
		store:      store,
	}

	consensus.RegisterListener(newBundleListener{res})

	return res, nil
}

// GetRpcService returns the RPC service of the node. If this node does not
// expose an RPC service, it returns nil.
func (n *Node) GetRpcService() rpc.Server {
	return n.rpc
}

func (n *Node) onNewBundle(bundle types.Bundle) {
	n.store.AddBlock(n.chainState.Apply(bundle.Transactions))
}

type newBundleListener struct {
	node *Node
}

func (l newBundleListener) OnNewBundle(bundle types.Bundle) {
	l.node.onNewBundle(bundle)
}

type rpcBackendAdapter struct {
	txpool.TxPool
	*store.Store
}

type payloadSourceAdapter struct {
	state state.State
	pool  txpool.TxPool
}

func (a *payloadSourceAdapter) GetNextBlockNumber() uint32 {
	return a.state.GetCurrentBlockNumber() + 1
}

func (a *payloadSourceAdapter) GetCandidateTransactions() []types.Transaction {
	return a.pool.GetExecutableTransactions(nonceSourceAdapter{state: a.state})
}

type nonceSourceAdapter struct {
	state state.State
}

func (n nonceSourceAdapter) GetNonce(address types.Address) types.Nonce {
	account := n.state.GetAccount(address)
	return account.Nonce
}
