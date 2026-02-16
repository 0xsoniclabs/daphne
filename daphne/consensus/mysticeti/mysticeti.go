// Copyright 2026 Sonic Operations Ltd
// This file is part of the Daphne consensus development infrastructure for Sonic.
//
// Daphne is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Daphne is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Daphne. If not, see <http://www.gnu.org/licenses/>.

package mysticeti

import (
	"slices"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

// Reference to paper: https://docs.sui.io/paper/mysticeti.pdf

// Nomenclature Mapping:
//
// This implementation uses our proposed nomenclature with Mysticeti equivalents noted below:
//
//	Proposed Term          | Mysticeti Term
//	-----------------------|--------------------
//	Event                  | Block
//	Candidate              | Proposer Slot
//	Leader                 | Leader
//	Frame                  | Wave
//	Equivocation           | Equivocation
//	Reach                  | Observes
//	Strongly reach         | Certifies
//	Certifies              | Decides

// Round represents a consensus round number.
// Rounds start from 0 (genesis).
type Round uint32

// Compile-time interface compliance checks
var (
	_ layering.Factory  = Factory{}
	_ layering.Layering = (*Mysticeti)(nil)
)

// Factory configures and produces Mysticeti layering instances.
// Mysticeti uses state-based round transitions with timeouts.
//
// NOTE: This implementation assumes uniform stake distribution among validators
// (each validator has equal voting weight). Non-uniform stake is not currently supported.
type Factory struct {
	RoundTimeout time.Duration // timeout for waiting for leader's event (Δ in paper), defaults to 500ms
}

func (f Factory) String() string {
	return "mysticeti"
}

func (f Factory) NewLayering(dag model.Dag, committee *consensus.Committee) layering.Layering {
	validators := committee.Validators() // sorted deterministically

	// Verify uniform stake assumption: all validators must have equal stake
	// This implementation counts validators rather than summing stake for quorum checks
	if len(validators) > 0 {
		firstStake := committee.GetValidatorStake(validators[0])
		for _, v := range validators[1:] {
			if committee.GetValidatorStake(v) != firstStake {
				panic("mysticeti: non-uniform stake distribution not supported")
			}
		}
	}

	m := &Mysticeti{
		dag:                  dag,
		committee:            committee,
		validators:           validators,
		eventRounds:          make(map[model.EventId]uint32),
		slotDecisions:        make(map[uint32]layering.Verdict),
		lowestUndecidedRound: 0, // Start from round 0
	}
	return m
}

// Mysticeti implements the Mysticeti-C consensus layering.
// Based on the paper: "MYSTICETI: Reaching the Latency Limits with Uncertified DAGs"
// Reference to paper: https://docs.sui.io/paper/mysticeti.pdf
//
// Key concepts:
//   - Candidate: A tuple (validator, round) that can be certified, skipped, or undecided
//   - Reach: Event B' reaches event B ≡ (A, r, h) if in DFS from B', B is the FIRST event
//     encountered for validator A at round r (handles equivocation correctly) [Mysticeti: observes]
//   - Certificate pattern: 2f+1 events at round r+1 that reach an event at round r
//   - Skip pattern: 2f+1 events at round r+1 that do NOT reach any event for a candidate
//
// Certification rule (3 message rounds):
//   - Direct certification: 2f+1 certificates for the leader (events in r+2 that each have
//     2f+1 parents from r+1 reaching the leader) [Mysticeti: strongly reaches]
//   - Indirect certification: Via anchor - a later certified leader that has certified path to this leader
//
// Liveness: Validators wait Δ for the primary event before proceeding.
// Note: The RoundTimeout (Δ) is configured via Factory but used by the emitter/driver layer,
// not by this layering implementation directly.
type Mysticeti struct {
	dag        model.Dag
	committee  *consensus.Committee
	validators []consensus.ValidatorId // pre-sorted for leader schedule

	// eventRounds maps event IDs to their assigned rounds
	eventRounds map[model.EventId]uint32

	// slotDecisions caches the decision for each candidate (round -> verdict)
	// This implements the certified/skipped/undecided state machine [Mysticeti: to-commit/to-skip/undecided]
	slotDecisions map[uint32]layering.Verdict

	// lowestUndecidedRound tracks the earliest round that hasn't been fully decided.
	// This enables cascading indirect decisions: when a later leader is certified,
	// it can serve as an anchor to decide earlier undecided leaders.
	lowestUndecidedRound uint32
}

// getRoundLeader returns the single leader validator for round r.
// Round-robin assignment: leader for round r is validator at index r % n
// This matches GETPREDEFINEDPROPOSER from Algorithm 2 in the paper.
func (m *Mysticeti) getRoundLeader(round uint32) consensus.ValidatorId {
	n := len(m.validators)
	if n == 0 {
		var empty consensus.ValidatorId
		return empty
	}
	leaderIdx := int(round) % n
	return m.validators[leaderIdx]
}

// isRoundLeader checks if validator is the leader for round r.
func (m *Mysticeti) isRoundLeader(round uint32, validator consensus.ValidatorId) bool {
	return m.getRoundLeader(round) == validator
}

// reaches checks if voter "reaches" candidate [Mysticeti: supports/observes].
//
// Definition:
// "We say that an event B' reaches a past event B ≡ (A, r, h) if, in the depth-first search
// performed starting at B' and recursively following all events in the sequence of events
// hashed, event B is the FIRST event encountered for validator A at round r."
//
// This is critical for handling equivocation correctly: if validator A equivocates at round r
// (produces multiple events), each subsequent event can only "reach" one of them.
func (m *Mysticeti) reaches(voter, candidate *model.Event) bool {
	if voter == nil || candidate == nil {
		return false
	}

	candidateAuthor := candidate.Creator()
	candidateRound := m.GetRound(candidate)

	// DFS from voter to find the first event from candidateAuthor at candidateRound
	var firstEncountered *model.Event

	voter.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
		eventRound := m.GetRound(e)

		// Check if this is an event from candidateAuthor at candidateRound
		if e.Creator() == candidateAuthor && eventRound == candidateRound {
			firstEncountered = e
			return model.Visit_Abort
		}

		// Prune if we're below the candidate round
		if eventRound < candidateRound {
			return model.Visit_Prune
		}

		return model.Visit_Descent
	}))

	// Voter reaches candidate if the first encountered event is the candidate itself
	return firstEncountered != nil && firstEncountered.EventId() == candidate.EventId()
}

// IsCandidate returns true if event is a leader's event.
// In Mysticeti-C, every round has exactly one leader (single candidate per round).
func (m *Mysticeti) IsCandidate(event *model.Event) bool {
	if event == nil || !slices.Contains(m.validators, event.Creator()) {
		return false
	}

	round := m.GetRound(event)
	return m.isRoundLeader(round, event.Creator())
}

// IsLeader implements the Mysticeti-C certification rule.
//
// The decision rule operates in three steps:
// 1. Direct decision rule: Check for certificate or skip patterns
// 2. Indirect decision rule: Use anchor to decide undecided candidates
// 3. Certification sequence: Iterate candidates in order, certify/skip until first undecided
//
// A leader L in round r is certified when:
// - Direct: 2f+1 events in r+2 are certificates for L (each has 2f+1 parents from r+1 reaching L) [Mysticeti: strongly reaches]
// - Indirect: An anchor leader at round > r+2 is certified and has certified path to L
//
// A leader is skipped when:
// - Direct: 2f+1 events in r+1 do NOT reach L (skip pattern)
// - Indirect: Anchor is certified but no certified path to L
func (m *Mysticeti) IsLeader(event *model.Event) layering.Verdict {
	if !m.IsCandidate(event) {
		return layering.VerdictNo
	}

	eventRound := m.GetRound(event)
	quorum := m.committee.Quorum()

	// Process rounds from lowestUndecidedRound upward to enable cascading decisions
	// This implements the commit sequence from the paper
	for round := m.lowestUndecidedRound; round <= eventRound; round++ {
		m.tryDecideRoundLeader(round, quorum)
	}

	// Return the verdict for this specific event
	return m.getLeaderVerdict(event, eventRound, quorum)
}

// tryDecideRoundLeader attempts to decide the leader in a given round.
// If decided, advances lowestUndecidedRound.
func (m *Mysticeti) tryDecideRoundLeader(round uint32, quorum uint32) {
	// Check if already decided - return early if so
	if _, decided := m.slotDecisions[round]; decided {
		// Note: lowestUndecidedRound should already be > round if decision was cached,
		// since we update it when caching the decision below. No need to update here.
		return
	}

	leader := m.findLeaderInRound(round)
	if leader == nil {
		// No leader block in this round yet, cannot decide
		return
	}

	verdict := m.computeLeaderVerdict(leader, round, quorum)
	if verdict != layering.VerdictUndecided {
		m.slotDecisions[round] = verdict
		if round == m.lowestUndecidedRound {
			m.lowestUndecidedRound = round + 1
		}
	}
}

// getLeaderVerdict returns the verdict for a specific leader event.
func (m *Mysticeti) getLeaderVerdict(leader *model.Event, leaderRound uint32, quorum uint32) layering.Verdict {
	// Check cached decision
	if verdict, ok := m.slotDecisions[leaderRound]; ok {
		return verdict
	}

	return m.computeLeaderVerdict(leader, leaderRound, quorum)
}

// computeLeaderVerdict evaluates a single leader using direct and indirect decision rules.
func (m *Mysticeti) computeLeaderVerdict(leader *model.Event, leaderRound uint32, quorum uint32) layering.Verdict {
	// Try direct decision first (Algorithm 2: TRYDIRECTDECIDE)
	directVerdict := m.directDecision(leader, leaderRound, quorum)
	if directVerdict != layering.VerdictUndecided {
		return directVerdict
	}

	// Try indirect decision (Algorithm 3: TRYINDIRECTDECIDE)
	return m.indirectDecision(leader, leaderRound, quorum)
}

// directDecision implements the direct decision rule from Algorithm 2 in the paper.
//
// Decision outcomes:
// - Certified: 2f+1 certificates exist for the leader [Mysticeti: to-commit]
// - Skipped: Skip pattern exists (2f+1 events in r+1 don't reach the leader) [Mysticeti: to-skip]
// - Undecided: Neither pattern observed yet
func (m *Mysticeti) directDecision(leader *model.Event, leaderRound uint32, quorum uint32) layering.Verdict {
	// Check for skip pattern first (Algorithm 1: SKIPPEDPROPOSER)
	if m.hasSkipPattern(leader, leaderRound, quorum) {
		return layering.VerdictNo
	}

	// Check for commit pattern (Algorithm 1: SUPPORTEDPROPOSER)
	if m.hasCommitPattern(leader, leaderRound, quorum) {
		return layering.VerdictYes
	}

	return layering.VerdictUndecided
}

// hasSkipPattern checks if the skip pattern exists for a leader.
//
// From paper Algorithm 1 (SKIPPEDPROPOSER):
// A candidate is skipped if 2f+1 events in r+1 have NO direct parent from the leader's
// author at the leader's round. This correctly handles equivocation: if any parent is
// from the leader's author at the leader's round, the voting block does NOT contribute
// to the skip pattern (since some block from that author was seen).
//
// Note: This counts distinct validators, not stake. Assumes uniform stake distribution.
func (m *Mysticeti) hasSkipPattern(leader *model.Event, leaderRound uint32, quorum uint32) bool {
	nextRound := leaderRound + 1
	leaderAuthor := leader.Creator()

	noParentFromAuthor := uint32(0)
	seenValidators := make(map[consensus.ValidatorId]bool)

	for _, head := range m.dag.GetHeads() {
		head.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			eventRound := m.GetRound(e)

			if eventRound < nextRound {
				return model.Visit_Prune
			}
			if eventRound == nextRound && !seenValidators[e.Creator()] {
				seenValidators[e.Creator()] = true

				// Check if voting block has ANY direct parent from leader's author at leader's round
				hasParentFromAuthor := false
				for _, parent := range e.Parents() {
					parentRound := m.GetRound(parent)
					if parent.Creator() == leaderAuthor && parentRound == leaderRound {
						hasParentFromAuthor = true
						break
					}
				}

				if !hasParentFromAuthor {
					noParentFromAuthor++
				}
			}
			return model.Visit_Descent
		}))
	}

	// Skip pattern: quorum of events in r+1 have no parent from leader's author
	return noParentFromAuthor >= quorum
}

// hasCommitPattern checks if the certification pattern (2f+1 certificates) exists for a leader.
//
// From paper Algorithm 1 (SUPPORTEDPROPOSER):
// A leader is certified when 2f+1 events in r+2 are certificates for it [Mysticeti: commits].
// An event in r+2 is a certificate if it has 2f+1 parents from r+1 that reach the leader [Mysticeti: strongly reaches].
//
// This is the key difference from the original implementation:
// - We check if events in r+2 form certificates (ISCERT), not just if they reach certifying events
func (m *Mysticeti) hasCommitPattern(leader *model.Event, leaderRound uint32, quorum uint32) bool {
	decisionRound := leaderRound + 2
	certificateCount := uint32(0)

	seenValidators := make(map[consensus.ValidatorId]bool)

	for _, head := range m.dag.GetHeads() {
		head.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			eventRound := m.GetRound(e)

			if eventRound < decisionRound {
				return model.Visit_Prune
			}
			if eventRound == decisionRound && !seenValidators[e.Creator()] {
				seenValidators[e.Creator()] = true
				if m.isCertificate(e, leader, quorum) {
					certificateCount++
				}
			}
			return model.Visit_Descent
		}))
	}

	return certificateCount >= quorum
}

// isCertificate checks if an event is a certificate for a leader (i.e., strongly reaches the leader).
//
// From paper Algorithm 1 (ISCERT):
// An event certEvent is a certificate for leader if it has 2f+1 parents that vote (reach) for leader.
// This implements the "strongly reaches" relationship [Mysticeti: certifies].
//
// Note: This counts distinct validators, not stake. Assumes uniform stake distribution.
func (m *Mysticeti) isCertificate(certEvent, leader *model.Event, quorum uint32) bool {
	// Count parents that vote (reach) for the leader
	// Per paper: res ← |{b ∈ bcert.parents : IsVote(b, bproposer)}|
	reachCount := uint32(0)
	seenVoters := make(map[consensus.ValidatorId]bool)

	for _, parent := range certEvent.Parents() {
		if !seenVoters[parent.Creator()] && m.reaches(parent, leader) {
			seenVoters[parent.Creator()] = true
			reachCount++
		}
	}

	return reachCount >= quorum
}

// indirectDecision implements the indirect decision rule from Algorithm 3.
//
// From paper Section III-B (Step 2):
// 1. Find anchor: first leader at round > r+2 that is certified or undecided [Mysticeti: to-commit]
// 2. If anchor is undecided → candidate is undecided
// 3. If anchor is certified:
//   - If candidate has certificate pattern AND that pattern links to anchor → certified
//   - Otherwise → skipped
func (m *Mysticeti) indirectDecision(candidate *model.Event, candidateRound uint32, quorum uint32) layering.Verdict {
	// Find anchor: first leader at round > candidateRound+2 that is certified or undecided
	anchor, anchorVerdict := m.findAnchor(candidateRound+3, quorum)
	if anchor == nil {
		return layering.VerdictUndecided
	}

	// If anchor is undecided, we cannot decide candidate
	if anchorVerdict == layering.VerdictUndecided {
		return layering.VerdictUndecided
	}

	// Anchor is certified. Check conditions (Algorithm 1: CERTIFIEDLINK):
	// (a) Does candidate have a certificate pattern? (2f+1 events in r+1 reach it)
	if !m.hasCertificatePattern(candidate, candidateRound, quorum) {
		return layering.VerdictNo
	}

	// (b) Does the certificate pattern have a certified link to the anchor?
	if m.hasCertifiedLinkToAnchor(candidate, candidateRound, anchor, quorum) {
		return layering.VerdictYes
	}

	return layering.VerdictNo
}

// hasCertificatePattern checks if a leader has 2f+1 events in r+1 that reach it.
func (m *Mysticeti) hasCertificatePattern(leader *model.Event, leaderRound uint32, quorum uint32) bool {
	nextRound := leaderRound + 1
	reachCount := uint32(0)

	seenValidators := make(map[consensus.ValidatorId]bool)

	for _, head := range m.dag.GetHeads() {
		head.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			eventRound := m.GetRound(e)

			if eventRound < nextRound {
				return model.Visit_Prune
			}
			if eventRound == nextRound && !seenValidators[e.Creator()] {
				seenValidators[e.Creator()] = true
				if m.reaches(e, leader) {
					reachCount++
				}
			}
			return model.Visit_Descent
		}))
	}

	return reachCount >= quorum
}

// hasCertifiedLinkToAnchor checks if there's a certified link from candidate to anchor.
//
// From paper Algorithm 1 (CERTIFIEDLINK):
// Returns true if there exists a certificate for candidate that links to anchor.
func (m *Mysticeti) hasCertifiedLinkToAnchor(candidate *model.Event, candidateRound uint32, anchor *model.Event, quorum uint32) bool {
	decisionRound := candidateRound + 2

	// Find certificates for candidate in decision round
	for _, head := range m.dag.GetHeads() {
		var found bool
		head.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			eventRound := m.GetRound(e)

			if eventRound < decisionRound {
				return model.Visit_Prune
			}

			if eventRound == decisionRound && m.isCertificate(e, candidate, quorum) {
				// Check if anchor links to this certificate
				if m.dag.Reaches(anchor, e) {
					found = true
					return model.Visit_Abort
				}
			}
			return model.Visit_Descent
		}))
		if found {
			return true
		}
	}

	return false
}

// findAnchor finds the first leader at round >= startRound that is certified or undecided [Mysticeti: to-commit].
// Returns nil if no suitable anchor exists within the observed DAG bounds.
func (m *Mysticeti) findAnchor(startRound uint32, quorum uint32) (*model.Event, layering.Verdict) {
	// Find max round in DAG to bound the search
	maxObservedRound := uint32(0)
	for _, head := range m.dag.GetHeads() {
		headRound := m.GetRound(head)
		if headRound > maxObservedRound {
			maxObservedRound = headRound
		}
	}

	// Bounded search for leaders starting from startRound
	for round := startRound; round <= maxObservedRound; round++ {
		leader := m.findLeaderInRound(round)
		if leader == nil {
			// No leader in this round, try next
			continue
		}

		// Check cached decision first
		if verdict, ok := m.slotDecisions[round]; ok {
			if verdict == layering.VerdictYes || verdict == layering.VerdictUndecided {
				return leader, verdict
			}
			// Skipped, continue to next round
			continue
		}

		// Compute direct decision for potential anchor
		verdict := m.directDecision(leader, round, quorum)
		if verdict == layering.VerdictYes || verdict == layering.VerdictUndecided {
			return leader, verdict
		}
		// Verdict is No (skipped), try next round
	}

	// No suitable anchor found within DAG bounds
	return nil, layering.VerdictUndecided
}

// findLeaderInRound finds the leader event in the given round.
// Returns nil if no leader event exists in that round.
func (m *Mysticeti) findLeaderInRound(round uint32) *model.Event {
	leader := m.getRoundLeader(round)
	var leaderEvent *model.Event

	for _, head := range m.dag.GetHeads() {
		head.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			eventRound := m.GetRound(e)

			if eventRound < round {
				return model.Visit_Prune
			}
			if eventRound == round && e.Creator() == leader {
				leaderEvent = e
				return model.Visit_Abort
			}
			return model.Visit_Descent
		}))
		if leaderEvent != nil {
			break
		}
	}

	return leaderEvent
}

// SortLeaders orders the provided events by their rounds, filtering out non-leaders.
// Events are sorted by round (only one leader per round in Mysticeti).
func (m *Mysticeti) SortLeaders(events []*model.Event) []*model.Event {
	leaders := slices.DeleteFunc(events, func(event *model.Event) bool {
		return m.IsLeader(event) != layering.VerdictYes
	})

	slices.SortFunc(leaders, func(a, b *model.Event) int {
		roundA := m.GetRound(a)
		roundB := m.GetRound(b)
		return int(roundA) - int(roundB)
	})

	return leaders
}

// GetRound extracts the round number from an event.
// Implements the Layering interface.
// If the event doesn't have an assigned round, it determines and assigns one.
//
// Per paper Section II-C:
// - Events with no parents are in round 0 (genesis)
// - Round = max(parent rounds) + 1
// - An event must include at least 2f+1 distinct hashes of events from the previous round
// - The first hash must be to the validator's own previous event (self-reference)
func (m *Mysticeti) GetRound(event *model.Event) uint32 {
	if event == nil {
		return 0
	}

	// Check if round is already cached
	if round, ok := m.eventRounds[event.EventId()]; ok {
		return round
	}

	// Determine round based on DAG structure
	parents := event.Parents()
	var round uint32
	if len(parents) == 0 {
		round = 0 // genesis round
	} else {
		// Round = max(parent rounds) + 1
		maxParentRound := uint32(0)
		for _, parent := range parents {
			maxParentRound = max(maxParentRound, m.GetRound(parent))
		}
		round = maxParentRound + 1
	}

	// Cache the result
	m.eventRounds[event.EventId()] = round
	return round
}
