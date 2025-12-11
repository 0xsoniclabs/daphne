package txpool

import (
	"cmp"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/types"
)

//go:generate mockgen -source lineup.go -destination=lineup_mock.go -package=txpool

// Lineup is a utility type organizing transactions ready to be processed in a
// form that allows for controlled consumption. It is used by the transaction
// pool to summarize executable transactions for inclusion in bundles by the
// consensus protocol.
type Lineup interface {
	// Filter allows a [LineupFilter] to select transactions for being included
	// in an execution set. Transactions contained in the Lineup are presented
	// to the filter in a valid execution order, respecting nonce ordering per
	// sender. The filter can decide to accept or reject transactions or abort
	// the entire filtering. If a transaction is accepted, follow-up
	// transactions of the same sender are considered. If a transaction is
	// rejected, follow-up transactions from the same sender are skipped. If the
	// filtering is aborted, no further transactions are processed.
	//
	// The method returns the list of transactions that were accepted by the
	// filter, in the order they were processed.
	Filter(LineupFilter) []types.Transaction

	// All returns all transactions in the lineup in a valid execution order,
	// respecting nonce ordering per sender. This is equivalent to calling
	// the Filter method with a filter accepting all transactions.
	All() []types.Transaction
}

// LineupFilter is an entity that filters transactions from a [Lineup].
type LineupFilter interface {
	Filter(types.Transaction) LineupDecision
}

// LineupDecision represents the decision made by a [LineupFilter]
// when processing a transaction from a [Lineup].
type LineupDecision bool

func (d LineupDecision) String() string {
	if d == LineupAccept {
		return "Accept"
	}
	return "Reject"
}

const (
	// LineupAccept indicates that the transaction is accepted and its effects
	// of an increased nonce should be considered for subsequent transactions
	// from the same sender.
	LineupAccept LineupDecision = true
	// LineupReject indicates that the transaction is rejected and subsequent
	// transactions from the same sender should be skipped.
	LineupReject LineupDecision = false
)

// -- Utility to wrap functions as LineupConsumers --

func WrapLineupFilter(
	fn func(types.Transaction) LineupDecision,
) LineupFilter {
	return lineupConsumerFunc(fn)
}

type lineupConsumerFunc func(types.Transaction) LineupDecision

func (f lineupConsumerFunc) Filter(tx types.Transaction) LineupDecision {
	return f(tx)
}

// -- lineup implementation --

type lineup struct {
	// Transactions in the lineup, grouped by sender, sorted by nonce
	transactions map[types.Address][]types.Transaction
}

// Filter presents the transactions in the lineup to the given filter for
// processing. The filter's decisions determine how the filter process proceeds.
func (l *lineup) Filter(filter LineupFilter) []types.Transaction {
	var accepted []types.Transaction
	for _, txs := range l.transactions {
		for _, tx := range txs {
			if filter != nil {
				decision := filter.Filter(tx)
				if decision == LineupReject {
					break
				}
			}
			accepted = append(accepted, tx)
		}
	}
	return accepted
}

func (l *lineup) All() []types.Transaction {
	return l.Filter(nil)
}

// NewLineup creates a new Lineup from the provided transactions. The resulting
// Lineup contains only consecutive sequences of transactions per sender.
// Duplicate nonces are removed, and any transaction that would create a gap in
// the nonce sequence for a sender is ignored.
func NewLineup(transactions []types.Transaction) *lineup {
	grouped := map[types.Address][]types.Transaction{}
	for _, tx := range transactions {
		grouped[tx.From] = append(grouped[tx.From], tx)
	}
	for _, group := range grouped {
		slices.SortFunc(group, func(a, b types.Transaction) int {
			return cmp.Compare(a.Nonce, b.Nonce)
		})
	}
	return newLineup(grouped)
}

// newLineup creates a new Lineup from the provided transactions grouped by
// sender. Is is assumed (but not checked) that the transactions for each sender
// are sorted by nonce in ascending order.
func newLineup(
	transactions map[types.Address][]types.Transaction,
) *lineup {
	// Create a deep-copy and make sure that there are no duplicates or gaps
	// in the nonces for each sender.
	res := map[types.Address][]types.Transaction{}
	for sender, txs := range transactions {
		if len(txs) == 0 {
			continue
		}
		group := make([]types.Transaction, 0, len(txs))
		group = append(group, txs[0])
		for i := 1; i < len(txs); i++ {
			last := group[len(group)-1]
			next := txs[i]
			if next.Nonce == last.Nonce {
				continue
			}
			if next.Nonce == last.Nonce+1 {
				group = append(group, next)
				continue
			}
			break
		}
		res[sender] = group
	}

	return &lineup{transactions: res}
}
