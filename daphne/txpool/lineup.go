package txpool

import (
	"github.com/0xsoniclabs/daphne/daphne/types"
)

//go:generate mockgen -source lineup.go -destination=lineup_mock.go -package=txpool
//go:generate stringer -type=LineupDecision -output lineup_string.go -trimprefix Lineup

// Lineup is a utility type organizing transactions ready to be processed in a
// form that allows for controlled consumption. It is used by the transaction
// pool to summarize executable transactions for inclusion in bundles by the
// consensus protocol.
type Lineup interface {
	// Process allows a [LineupFilter] to select transactions for being
	// included in an execution set. The Lineup presents contained transactions
	// in a valid execution order, respecting nonce ordering per sender, to the
	// filter. The
	// decider can decide to accept or reject transactions or abort the entire
	// processing. If a transaction is accepted, follow-up transactions of the
	// same sender are considered. If a transaction is rejected, follow-up
	// transactions from the same sender are skipped. If the processing is
	// aborted, no further transactions are processed.
	//
	// The method returns the list of transactions that were accepted by the
	// decider, in the order they were processed.
	Process(LineupFilter) []types.Transaction
}

// LineupDecision represents the decision made by a [LineupFilter]
// when processing a transaction from a [Lineup].
type LineupDecision int

const (
	// LineupAccept indicates that the transaction is accepted and its effects
	// of an increased nonce should be considered for subsequent transactions
	// from the same sender.
	LineupAccept LineupDecision = iota
	// LineupReject indicates that the transaction is rejected and subsequent
	// transactions from the same sender should be skipped.
	LineupReject
	// LineupAbort indicates that the consumption process should be aborted
	// immediately and no further transactions should be processed.
	LineupAbort
)

// LineupFilter is an entity that filters transactions from a [Lineup].
type LineupFilter interface {
	Filter(types.Transaction) LineupDecision
}

// -- Utility to wrap functions as LineupConsumers --

func WrapConsumer(
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

// Process presents the transactions in the lineup to the given consumer for
// processing. The consumer's decisions determine how the consumption proceeds.
func (l *lineup) Process(consumer LineupFilter) []types.Transaction {
	var accepted []types.Transaction
	for _, txs := range l.transactions {
		for _, tx := range txs {
			if consumer != nil {
				decision := consumer.Filter(tx)
				if decision == LineupAbort {
					return accepted
				}
				if decision == LineupReject {
					break
				}
			}
			accepted = append(accepted, tx)
		}
	}
	return accepted
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
