package single_proposer

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
)

// The code in this file is a copy of the proposer selection code from Sonic's
// single proposer implementation, adapted to fit into Daphne's codebase.
// The original implementation can be found [here]. The main adaptation are:
//  - Renaming "Frame" to "Round" to fit Daphne's terminology.
//  - Renaming validators to committee to fit Daphne's terminology.
//  - Epoch handling was removed as Daphne does not support epoch changes.
//
// [here]: https://github.com/0xsoniclabs/sonic/blob/main/inter/proposer_selection.go

// GetProposer returns the designated proposer for a given turn.
// The proposer is determined through deterministic sampling from the committee,
// proportional to the validator's stake.
func GetProposer(
	committee *consensus.Committee,
	turn Turn,
) (consensus.ValidatorId, error) {

	// The selection of the proposer for a given round is conducted as follows:
	//  1. f := sha256(epoch || turn) / 2^256, (where || is the concatenation operator)
	//  2. limit := f * total_weight
	//  3. from the list of validators sorted by their stake, find the first
	//     validator whose cumulative weight is greater than or equal to limit.

	// -- Preconditions --
	if len(committee.Validators()) == 0 {
		return 0, fmt.Errorf("no validators")
	}

	// Note that we use big.Rat to preserve precision in the division.
	// limit := (sha256(turn) * total_weight) / 2^256
	data := make([]byte, 0, 4)
	data = binary.BigEndian.AppendUint32(data, uint32(turn))
	hash := sha256.Sum256(data)
	limit := new(big.Rat).Quo(
		new(big.Rat).SetInt(
			new(big.Int).Mul(
				new(big.Int).SetBytes(hash[:]),
				big.NewInt(int64(committee.TotalStake())),
			),
		),
		new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 256)),
	)

	// Walk through the validators sorted by their stake (and ID as a tiebreaker)
	// and accumulate their weights until we reach the limit calculated above.
	members := committee.GetValidatorStakes()
	res := members[0].ValidatorId
	cumulated := big.NewRat(0, 1)
	for _, member := range members {
		cumulated.Num().Add(cumulated.Num(), big.NewInt(int64(member.Stake)))
		if cumulated.Cmp(limit) >= 0 {
			res = member.ValidatorId
			break
		}
	}
	return res, nil
}
