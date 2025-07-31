package txpool

import (
	"cmp"
	"fmt"
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

//go:generate mockgen -source txpool.go -destination=txpool_mock.go -package=txpool

// TxPool is the interface of a transaction pool used by nodes to manage
// transactions before they are included in Events and Blocks.
type TxPool interface {
	// Add the transaction to the pool. Error if unsuccessful.
	Add(tx types.Transaction) error
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

	mutex sync.Mutex
}

// NewTxPool instantiates a new transaction pool to manage transactions and
// to maintain and notify listeners for new transactions.
func NewTxPool() *txPool {
	return &txPool{
		transactions: make(map[types.Address][]types.Transaction),
		listeners:    make([]TxPoolListener, 0),
	}
}

// Add adds a new transaction to the pool. It returns and error if a transaction
// cannot be added due to nonce conflicts or for other reasons.
func (pool *txPool) Add(tx types.Transaction) error {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()

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

	// Notify all listeners about the new transaction.
	for _, listener := range pool.listeners {
		listener.OnNewTransaction(tx)
	}

	return nil
}

// GetExecutableTransactions returns transactions that can be executed for the
// given source address in NonceSource. Transactions that are outdated are
// pruned and only those with consecutive nonces starting from the latest nonce
// are returned.
func (pool *txPool) GetExecutableTransactions(
	source NonceSource) []types.Transaction {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()

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
// for the associaqted pool.
func (pool *txPool) RegisterListener(listener TxPoolListener) {
	if listener == nil {
		return
	}
	pool.listeners = append(pool.listeners, listener)
}
