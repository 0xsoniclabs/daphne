package single_proposer

import (
	"unsafe"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// SingleProposerPayload is the type if information communicated via events using the
// single proposer protocol to achieve consensus on bundles.
type SingleProposerPayload struct {
	// SyncState contains information tracking protocol state information to
	// determine who is allowed to propose the next bundle.
	SyncState SyncState
	// Proposal contains the proposed bundle, if any.
	Proposal *types.Bundle
}

func (p SingleProposerPayload) Size() uint32 {
	res := uint32(unsafe.Sizeof(p.SyncState))
	if p.Proposal != nil {
		res += p.Proposal.Size()
	}
	return res
}

func (p SingleProposerPayload) Clone() payload.Payload {
	var proposalClone *types.Bundle
	if p.Proposal != nil {
		proposalClone = p.Proposal.Clone()
	}
	return SingleProposerPayload{
		SyncState: p.SyncState,
		Proposal:  proposalClone,
	}
}
