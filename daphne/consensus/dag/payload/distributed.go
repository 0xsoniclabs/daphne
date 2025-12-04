package payload

import (
	"encoding/binary"
	"fmt"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

const (
	// NumRoundsBetweenReassignments is the number of rounds between transaction
	// responsibility re-assignments.
	NumRoundsBetweenReassignments = 64
	// NumNoncesPerPartition is the number of consecutive nonces assigned to a
	// single validator.
	NumNoncesPerPartition = 32
)

// DistributedProtocol is a payload protocol in which the right to propose
// transactions is sharded among multiple nodes. Nodes only include transactions
// they are responsible for proposing.
//
// This protocol is a simplified version of the one implemented in Sonic that
// can be found [here]. The following key-features are covered:
//   - transaction hash + nonce space partitioning among committee members
//   - periodic re-assignment of transaction responsibilities to nodes
//
// The following features are NOT covered:
//   - tracking of offline validators
//   - proportional assignment of transaction responsibilities based on stake
//   - tracking of in-flight transactions to avoid duplicates
//
// The following changes are made:
//   - the age of a transaction is not tracked by recording the first time a
//     transaction was seen ([txtime]); in Sonic, this is used to re-assign
//     responsibilities for transactions on a per-transaction basis. Here, we
//     use a global, event round based re-assignment of responsibilities for
//     simplicity.
//
// [here]: https://github.com/0xsoniclabs/sonic/blob/fd194b47e66b263651a6dbc0222fd64fdfc3e5f5/gossip/emitter/txs.go#L131-L159.
// [txtime]: https://github.com/0xsoniclabs/sonic/blob/fd194b47e66b263651a6dbc0222fd64fdfc3e5f5/utils/txtime/txtime.go
type DistributedProtocol struct {
	committee            *consensus.Committee
	localValidatorId     consensus.ValidatorId
	highestNonceProposed map[types.Address]types.Nonce
}

func (p DistributedProtocol) BuildPayload(
	info EventInfo,
	lineup txpool.Lineup,
) Transactions {

	// The current round determines the assignment of transaction responsibilities.
	round := info.GetRound() / NumRoundsBetweenReassignments

	// Select only the transactions for which this node is responsible.
	payload := lineup.Process(txpool.WrapConsumer(func(tx types.Transaction) txpool.LineupDecision {
		if p.isMyResponsibility(round, tx.From, tx.Nonce) {
			// Do not re-emit transactions with nonces lower than or equal to
			// the highest nonce we have already proposed for the sender.
			if last, found := p.highestNonceProposed[tx.From]; !found || tx.Nonce > last {
				p.highestNonceProposed[tx.From] = tx.Nonce
				return txpool.LineupAccept
			}
		}
		return txpool.LineupReject
	}))
	return payload
}

func (p DistributedProtocol) Merge(payloads []Transactions) []types.Bundle {
	var txs []types.Transaction
	for _, payload := range payloads {
		txs = append(txs, payload...)
	}
	sortTransactionsInExecutionOrder(txs)
	return []types.Bundle{{Transactions: txs}}
}

func (p DistributedProtocol) isMyResponsibility(
	round uint32,
	sender types.Address,
	nonce types.Nonce,
) bool {
	data := []byte{}
	data = binary.BigEndian.AppendUint32(data, round)
	data = binary.BigEndian.AppendUint64(data, uint64(sender))
	data = binary.BigEndian.AppendUint64(data, uint64(nonce/NumNoncesPerPartition))

	hash := types.Sha256(data)

	validators := p.committee.Validators()
	partition := binary.BigEndian.Uint32(hash[:4]) % uint32(len(validators))

	fmt.Printf("local: %d, partition: %d, hash: %x, round: %d, sender: %v, nonce: %d\n", p.localValidatorId, partition, hash[:4], round, sender, nonce)

	return validators[partition] == p.localValidatorId
}

type DistributedProtocolFactory struct{}

func (f DistributedProtocolFactory) NewProtocol(
	committee *consensus.Committee,
	localValidatorId consensus.ValidatorId,
) Protocol[Transactions] {
	return DistributedProtocol{
		committee:            committee,
		localValidatorId:     localValidatorId,
		highestNonceProposed: make(map[types.Address]types.Nonce),
	}
}

func (f DistributedProtocolFactory) String() string {
	return "distributed"
}
