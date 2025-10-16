package txpool

import (
	"cmp"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

//go:generate mockgen -source txpool.go -destination=txpool_mock.go -package=txpool

// TxPool is the interface of a transaction pool used by nodes to manage
// transactions before they are included in Events and Blocks.
// All operations are thread-safe.
type TxPool interface {
	// Add the transaction to the pool. Error if unsuccessful.
	Add(tx types.Transaction) error

	// Contains checks whether the given transaction is currently pooled.
	Contains(hash types.Hash) bool

	// GetExecutableTransactions returns transactions that can be executed
	// from the latest nonce and onwards by the source address in NonceSource.
	// Transactions that are outdated are ignored and only those with con-
	// secutive nonces starting from the latest nonce are returned. Transactions
	// that are outdated are also pruned (removed) from the pool.
	GetExecutableTransactions(NonceSource) []types.Transaction
	// RegisterListener registers a listener to be notified of new transactions
	// added to the pool.
	RegisterListener(TxPoolListener)
}

// NonceSource is an interface that provides the latest nonce for a given
// source address.
type NonceSource interface {
	GetNonce(address types.Address) types.Nonce
}

// TxPoolListener is an interface for listening to new transactions added to
// some transaction pool.
type TxPoolListener interface {
	OnNewTransaction(tx types.Transaction)
}

// txPool is an implementation of TxPool that manages transactions and notifies
// listeners about new transactions.
type txPool struct {
	// Transactions grouped by sender, sorted by nonce
	transactions map[types.Address][]types.Transaction
	listeners    []TxPoolListener

	transactionMutex sync.Mutex
	listenersMutex   sync.Mutex

	tracker tracker.Tracker
}

// NewTxPool instantiates a new transaction pool to manage transactions and
// to maintain and notify listeners for new transactions.
// The given tracker is used to track transaction life cycles.
func NewTxPool(tracker tracker.Tracker) *txPool {
	return &txPool{
		transactions: make(map[types.Address][]types.Transaction),
		listeners:    make([]TxPoolListener, 0),
		tracker:      tracker,
	}
}

// Add adds a new transaction to the pool and notifies registered listeners if
// the addition is successful. It returns an error if a transaction cannot be
// added due to nonce conflicts or for other reasons.
func (pool *txPool) Add(tx types.Transaction) error {
	if err := pool.add(tx); err != nil {
		return err
	}

	pool.listenersMutex.Lock()
	defer pool.listenersMutex.Unlock()

	// Notify all listeners about the new transaction.
	for _, listener := range pool.listeners {
		listener.OnNewTransaction(tx)
	}
	return nil
}

func (pool *txPool) add(tx types.Transaction) error {
	pool.transactionMutex.Lock()
	defer pool.transactionMutex.Unlock()

	// Ignore the new transaction if there is already a transaction with the
	// same nonce from the same sender.
	present := pool.transactions[tx.From]
	if slices.ContainsFunc(present, func(t types.Transaction) bool {
		return t.Nonce == tx.Nonce
	}) {
		return fmt.Errorf("transaction with nonce %d from %s already exists",
			tx.Nonce, tx.From)
	}

	// Add transaction to the pool, grouped by sender address.
	pool.transactions[tx.From] = append(pool.transactions[tx.From], tx)

	// Sort the transactions by nonce for the same sender.
	slices.SortFunc(pool.transactions[tx.From], func(a, b types.Transaction) int {
		return cmp.Compare(a.Nonce, b.Nonce)
	})

	if pool.tracker != nil {
		pool.tracker.Track(mark.TxAddedToPool, "hash", tx.Hash())
	}

	return nil
}

// Contains checks whether the given transaction is currently pooled.
func (pool *txPool) Contains(hash types.Hash) bool {
	pool.transactionMutex.Lock()
	defer pool.transactionMutex.Unlock()

	for _, group := range pool.transactions {
		if slices.ContainsFunc(group, func(tx types.Transaction) bool {
			return tx.Hash() == hash
		}) {
			return true
		}
	}
	return false
}

// GetExecutableTransactions returns transactions that can be executed for the
// given source address in NonceSource. Transactions that are outdated are
// pruned and only those with consecutive nonces starting from the latest nonce
// are returned.
func (pool *txPool) GetExecutableTransactions(
	source NonceSource) []types.Transaction {
	pool.transactionMutex.Lock()
	defer pool.transactionMutex.Unlock()

	var txs []types.Transaction
	for sender, group := range pool.transactions {
		cur := source.GetNonce(sender)

		// Prune all transactions with out-dated nonces.
		for len(group) > 0 && group[0].Nonce < cur {
			group = group[1:]
		}
		pool.transactions[sender] = group

		// Propose all transactions with consecutive nonces.
		for ; len(group) > 0 && group[0].Nonce == cur; cur++ {
			txs = append(txs, group[0])
			group = group[1:]
		}
	}
	return txs
}

// RegisterListener registers a listener to be notified of new transactions
// for the associated pool.
func (pool *txPool) RegisterListener(listener TxPoolListener) {
	if listener == nil {
		return
	}
	pool.listenersMutex.Lock()
	defer pool.listenersMutex.Unlock()
	pool.listeners = append(pool.listeners, listener)
}

// InstallTxGossip installs a synchronization protocol automatically keeping the
// given pool in sync with other pools on the P2P network running the same protocol.
func InstallTxGossip(pool TxPool, p2pServer p2p.Server) {
	installTxGossip(pool, p2pServer)
}

// installTxGossip is a helper function that returns a gossip protocol,
// useful for testing purposes.
func installTxGossip(pool TxPool, p2pServer p2p.Server) generic.Broadcaster[types.Transaction] {
	txGossip := generic.NewGossip(p2pServer, func(tx types.Transaction) types.Hash {
		return tx.Hash()
	})
	pool.RegisterListener(poolListenerAdapter{txGossip})
	txGossip.RegisterReceiver(generic.WrapBroadcastReceiver(func(message types.Transaction) {
		if err := pool.Add(message); err != nil {
			slog.Debug("Received transaction not added to pool", "sender", message.From,
				"transaction hash", message.Hash(), "reason", err)
		}
	}))
	return txGossip
}

type poolListenerAdapter struct {
	generic.Broadcaster[types.Transaction]
}

func (a poolListenerAdapter) OnNewTransaction(tx types.Transaction) {
	a.Broadcast(tx)
}
