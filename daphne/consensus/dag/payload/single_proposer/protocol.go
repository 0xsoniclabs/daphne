package single_proposer

import (
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

type SingleProposerProtocol struct {
	committee        *consensus.Committee
	localValidatorId consensus.ValidatorId
	proposalTracker  ProposalTracker
}

func (p *SingleProposerProtocol) OnConnectedEventPayload(
	_ consensus.ValidatorId,
	round uint32,
	payload SingleProposerPayload,
) {
	if payload.Proposal == nil {
		return
	}
	p.proposalTracker.RegisterSeenProposal(
		Round(round),
		BlockNumber(payload.Proposal.Number),
	)
}

func (p *SingleProposerProtocol) BuildPayload(
	meta payload.EventMeta[SingleProposerPayload],
	provider consensus.TransactionProvider,
) SingleProposerPayload {

	// This is a simplified version of Sonic's single proposer payload building
	// logic found here: https://github.com/0xsoniclabs/sonic/blob/8afbf62f81e82344f2bc0dab91be684e5b739f67/gossip/emitter/proposals.go#L85

	// Step 1: Join the sync states from the parent payloads to get the sync
	// state before the current event.
	syncState := SyncState{}
	for _, parentPayload := range meta.ParentPayloads {
		syncState = JoinSyncStates(syncState, parentPayload.SyncState)
	}

	// Step 2: Determine whether this validator is ready to propose the next bundle.
	nextBlock := BlockNumber(provider.GetCurrentBlockNumber() + 1)
	if p.proposalTracker.IsPending(Round(meta.ParentsMaxRound), nextBlock) {
		// This validator is not able to propose a new bundle yet; return only
		// the unmodified sync state.
		return SingleProposerPayload{SyncState: syncState}
	}

	// Step 3: Determine if the local validator is the proposer for the current turn.
	round := Round(meta.ParentsMaxRound)
	allowed, turn, err := IsAllowedToPropose(p.localValidatorId, p.committee, syncState, round)
	if err != nil || !allowed {
		// If not allowed to propose, return only the unmodified sync state.
		return SingleProposerPayload{SyncState: syncState}
	}

	// Step 4: Build the proposed bundle from the txpool lineup.
	// TODO: add test-running of transactions before proposing.
	return SingleProposerPayload{
		SyncState: SyncState{
			LastSeenProposalTurn:  turn,
			LastSeenProposalRound: round,
		},
		Proposal: &types.Bundle{
			Number:       uint32(nextBlock),
			Transactions: provider.GetCandidateLineup().All(),
		},
	}
}

func (p *SingleProposerProtocol) Merge(payloads []SingleProposerPayload) []types.Bundle {
	// This code is derived from Sonic's original implementation.
	// See: https://github.com/0xsoniclabs/sonic/blob/8afbf62f81e82344f2bc0dab91be684e5b739f67/gossip/c_block_callbacks.go#L751-L752

	// Payloads are merged by collecting all proposed bundles and returning them
	// in order of their turn numbers.
	var proposals []SingleProposerPayload
	for _, pl := range payloads {
		if pl.Proposal != nil {
			proposals = append(proposals, pl)
		}
	}

	// Sort proposals by turn number (ascending). Event formation / validation
	// rules would make sure there is at most one proposal per turn number in
	// the a confirmed DAG structure. Thus, sorting by turn number is sufficient.
	slices.SortFunc(proposals, func(a, b SingleProposerPayload) int {
		return int(a.SyncState.LastSeenProposalTurn) - int(b.SyncState.LastSeenProposalTurn)
	})

	// Extract bundles from proposals.
	var bundles []types.Bundle
	for _, pl := range proposals {
		bundles = append(bundles, *pl.Proposal)
	}
	return bundles
}

// --- Protocol Factory ---

// SingleProposerProtocolFactory is a factory for creating instances of the
// single proposer payload protocol.
type SingleProposerProtocolFactory struct {
}

func (f SingleProposerProtocolFactory) NewProtocol(
	committee *consensus.Committee,
	localValidatorId consensus.ValidatorId,
) payload.Protocol[SingleProposerPayload] {
	return &SingleProposerProtocol{
		committee:        committee,
		localValidatorId: localValidatorId,
	}
}

func (f SingleProposerProtocolFactory) String() string {
	return "single_proposer"
}

/*
// CalculateIncomingProposalSyncState aggregates the last seen proposal information
// from the event's parents.
func CalculateIncomingProposalSyncState(
	reader EventReader,
	event model.Event[Payload],
) ProposalSyncState {
	// The last seen proposal information of the parents needs to be aggregated.
	res := ProposalSyncState{}
	for _, parent := range event.Parents() {
		current := reader.GetEventPayload(parent).ProposalSyncState
		res = JoinProposalSyncStates(res, current)
	}
	return res
}
*/
