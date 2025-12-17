package mysticeti

import (
	"slices"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

const (
	DefaultRoundTimeout = 500 * time.Millisecond
)

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
type Factory struct {
	RoundTimeout time.Duration // timeout for waiting for leader's block (Δ in paper), defaults to 500ms
}

func (f Factory) String() string {
	return "mysticeti"
}

func (f Factory) NewLayering(dag model.Dag, committee *consensus.Committee) layering.Layering {
	roundTimeout := f.RoundTimeout
	if roundTimeout == 0 {
		roundTimeout = DefaultRoundTimeout
	}

	validators := committee.Validators() // sorted deterministically
	m := &Mysticeti{
		dag:                  dag,
		committee:            committee,
		validators:           validators,
		roundTimeout:         roundTimeout,
		eventRounds:          make(map[model.EventId]uint32),
		slotDecisions:        make(map[uint32]layering.Verdict),
		lowestUndecidedRound: 0, // Start from round 0
	}
	return m
}

// Mysticeti implements the Mysticeti-C consensus layering.
// Based on the paper: "MYSTICETI: Reaching the Latency Limits with Uncertified DAGs"
//
// Key concepts from the paper:
//   - Proposer slot: A tuple (validator, round) that can be to-commit, to-skip, or undecided
//   - Support: Block B' supports block B ≡ (A, r, h) if in DFS from B', B is the FIRST block
//     encountered for validator A at round r (handles equivocation correctly)
//   - Certificate pattern: 2f+1 blocks at round r+1 that support a block at round r
//   - Skip pattern: 2f+1 blocks at round r+1 that do NOT support any block for a slot
//
// Commit rule (3 message rounds):
//   - Direct commit: 2f+1 certificates for the leader (blocks in r+2 that each have
//     2f+1 parents from r+1 supporting the leader)
//   - Indirect commit: Via anchor - a later committed leader that has certified path to this leader
//
// Liveness: Validators wait Δ for the primary block before proceeding.
type Mysticeti struct {
	dag          model.Dag
	committee    *consensus.Committee
	validators   []consensus.ValidatorId // pre-sorted for leader schedule
	roundTimeout time.Duration           // Δ timeout for primary block waiting

	// eventRounds maps event IDs to their assigned rounds
	eventRounds map[model.EventId]uint32

	// slotDecisions caches the decision for each proposer slot (round -> verdict)
	// This implements the to-commit/to-skip/undecided state machine from the paper
	slotDecisions map[uint32]layering.Verdict

	// lowestUndecidedRound tracks the earliest round that hasn't been fully decided.
	// This enables cascading indirect decisions: when a later leader commits,
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

// supportsBlock checks if voter "supports" candidate per the paper's definition.
//
// From paper Section II-C:
// "We say that a block B' supports a past block B ≡ (A, r, h) if, in the depth-first search
// performed starting at B' and recursively following all blocks in the sequence of blocks
// hashed, block B is the FIRST block encountered for validator A at round r."
//
// This is critical for handling equivocation correctly: if validator A equivocates at round r
// (produces multiple blocks), each subsequent block can only "support" one of them.
func (m *Mysticeti) supportsBlock(voter, candidate *model.Event) bool {
	if voter == nil || candidate == nil {
		return false
	}

	candidateAuthor := candidate.Creator()
	candidateRound := m.GetRound(candidate)
	if candidateRound == 0 {
		return false
	}

	// DFS from voter to find the first block from candidateAuthor at candidateRound
	var firstEncountered *model.Event
	seen := make(map[model.EventId]bool)

	voter.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {

		if seen[e.EventId()] {
			return model.Visit_Prune
		}

		seen[e.EventId()] = true
		eventRound := m.GetRound(e)

		// Check if this is a block from candidateAuthor at candidateRound
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

	// Voter supports candidate if the first encountered block is the candidate itself
	return firstEncountered != nil && firstEncountered.EventId() == candidate.EventId()
}

// IsCandidate returns true if event is a leader's event.
// In Mysticeti-C, every round has exactly one leader (single proposer slot per round).
func (m *Mysticeti) IsCandidate(event *model.Event) bool {
	if event == nil || !slices.Contains(m.validators, event.Creator()) {
		return false
	}

	round := m.GetRound(event)
	if round == 0 {
		return false // Cannot determine round yet
	}

	return m.isRoundLeader(round, event.Creator())
}

// IsLeader implements the Mysticeti-C commit rule from the paper.
//
// From paper Section III-B:
// The decision rule operates in three steps:
// 1. Direct decision rule: Check for certificate or skip patterns
// 2. Indirect decision rule: Use anchor to decide undecided slots
// 3. Commit sequence: Iterate slots in order, commit/skip until first undecided
//
// A leader L in round r is committed when:
// - Direct: 2f+1 blocks in r+2 are certificates for L (each has 2f+1 parents from r+1 supporting L)
// - Indirect: An anchor leader at round > r+2 is committed and has certified path to L
//
// A leader is skipped when:
// - Direct: 2f+1 blocks in r+1 do NOT support L (skip pattern)
// - Indirect: Anchor is committed but no certified path to L
func (m *Mysticeti) IsLeader(event *model.Event) layering.Verdict {
	if !m.IsCandidate(event) {
		return layering.VerdictNo
	}

	eventRound := m.GetRound(event)
	if eventRound == 0 {
		return layering.VerdictNo
	}

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
	// Check if already decided
	if _, decided := m.slotDecisions[round]; decided {
		if round == m.lowestUndecidedRound {
			m.lowestUndecidedRound = round + 1
		}
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
// From paper Section III-B:
// - To-commit: 2f+1 commit patterns (certificates) exist for the leader
// - To-skip: Skip pattern exists (2f+1 blocks in r+1 don't support the leader)
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
// A slot is skipped if 2f+1 blocks in r+1 do NOT support any proposal for the slot.
// For equivocating validators, this means no proposal from them gets 2f+1 support.
func (m *Mysticeti) hasSkipPattern(leader *model.Event, leaderRound uint32, quorum uint32) bool {
	nextRound := leaderRound + 1
	nonSupportingCount := uint32(0)
	totalInNextRound := uint32(0)

	seenValidators := make(map[consensus.ValidatorId]bool)

	for _, head := range m.dag.GetHeads() {
		head.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			eventRound := m.GetRound(e)

			if eventRound == 0 || eventRound < nextRound {
				return model.Visit_Prune
			}
			if eventRound == nextRound && !seenValidators[e.Creator()] {
				seenValidators[e.Creator()] = true
				totalInNextRound++
				if !m.supportsBlock(e, leader) {
					nonSupportingCount++
				}
			}
			return model.Visit_Descent
		}))
	}

	// Skip pattern: have 2f+1 in r+1, and 2f+1 of them don't support the leader
	return totalInNextRound >= quorum && nonSupportingCount >= quorum
}

// hasCommitPattern checks if the commit pattern (2f+1 certificates) exists for a leader.
//
// From paper Algorithm 1 (SUPPORTEDPROPOSER):
// A leader is committed when 2f+1 blocks in r+2 are certificates for it.
// A block in r+2 is a certificate if it has 2f+1 parents from r+1 that support the leader.
//
// This is the key difference from the original implementation:
// - We check if blocks in r+2 form certificates (ISCERT), not just if they reach certifying events
func (m *Mysticeti) hasCommitPattern(leader *model.Event, leaderRound uint32, quorum uint32) bool {
	decisionRound := leaderRound + 2
	certificateCount := uint32(0)

	seenValidators := make(map[consensus.ValidatorId]bool)

	for _, head := range m.dag.GetHeads() {
		head.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			eventRound := m.GetRound(e)

			if eventRound == 0 || eventRound < decisionRound {
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

// isCertificate checks if a block is a certificate for a leader.
//
// From paper Algorithm 1 (ISCERT):
// A block bcert is a certificate for bproposer if it has 2f+1 parents that vote (support) for bproposer.
// The parents must be from round r+1 (one round before bcert).
func (m *Mysticeti) isCertificate(certBlock, leader *model.Event, quorum uint32) bool {
	certRound := m.GetRound(certBlock)
	leaderRound := m.GetRound(leader)

	// Certificate must be in leader round + 2
	if certRound != leaderRound+2 {
		return false
	}

	// Count parents from r+1 that support the leader
	supportCount := uint32(0)
	seenVoters := make(map[consensus.ValidatorId]bool)

	for _, parent := range certBlock.Parents() {
		parentRound := m.GetRound(parent)

		// Only count parents from r+1 (the voting round)
		if parentRound == leaderRound+1 && !seenVoters[parent.Creator()] {
			if m.supportsBlock(parent, leader) {
				seenVoters[parent.Creator()] = true
				supportCount++
			}
		}
	}

	return supportCount >= quorum
}

// indirectDecision implements the indirect decision rule from Algorithm 3.
//
// From paper Section III-B (Step 2):
// 1. Find anchor: first leader at round > r+2 that is to-commit or undecided
// 2. If anchor is undecided → candidate is undecided
// 3. If anchor is to-commit:
//   - If candidate has certificate pattern AND that pattern links to anchor → to-commit
//   - Otherwise → to-skip
func (m *Mysticeti) indirectDecision(candidate *model.Event, candidateRound uint32, quorum uint32) layering.Verdict {
	// Find anchor: first leader at round > candidateRound+2 that is to-commit or undecided
	anchor, anchorVerdict := m.findAnchor(candidateRound+3, quorum)
	if anchor == nil {
		return layering.VerdictUndecided
	}

	// If anchor is undecided, we cannot decide candidate
	if anchorVerdict == layering.VerdictUndecided {
		return layering.VerdictUndecided
	}

	// Anchor is to-commit. Check conditions (Algorithm 1: CERTIFIEDLINK):
	// (a) Does candidate have a certificate pattern? (2f+1 blocks in r+1 support it)
	if !m.hasCertificatePattern(candidate, candidateRound, quorum) {
		return layering.VerdictNo
	}

	// (b) Does the certificate pattern have a certified link to the anchor?
	if m.hasCertifiedLinkToAnchor(candidate, candidateRound, anchor, quorum) {
		return layering.VerdictYes
	}

	return layering.VerdictNo
}

// hasCertificatePattern checks if a leader has 2f+1 blocks in r+1 that support it.
func (m *Mysticeti) hasCertificatePattern(leader *model.Event, leaderRound uint32, quorum uint32) bool {
	nextRound := leaderRound + 1
	supportCount := uint32(0)

	seenValidators := make(map[consensus.ValidatorId]bool)

	for _, head := range m.dag.GetHeads() {
		head.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			eventRound := m.GetRound(e)

			if eventRound == 0 || eventRound < nextRound {
				return model.Visit_Prune
			}
			if eventRound == nextRound && !seenValidators[e.Creator()] {
				seenValidators[e.Creator()] = true
				if m.supportsBlock(e, leader) {
					supportCount++
				}
			}
			return model.Visit_Descent
		}))
	}

	return supportCount >= quorum
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
			if found {
				return model.Visit_Abort
			}

			eventRound := m.GetRound(e)

			if eventRound == 0 || eventRound < decisionRound {
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

// findAnchor finds the first leader at round >= startRound that is to-commit or undecided.
func (m *Mysticeti) findAnchor(startRound uint32, quorum uint32) (*model.Event, layering.Verdict) {
	// Search for leaders starting from startRound
	for round := startRound; ; round++ {
		leader := m.findLeaderInRound(round)
		if leader == nil {
			// No more leaders in DAG
			return nil, layering.VerdictUndecided
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
}

// findLeaderInRound finds the leader block in the given round.
// Returns nil if no leader block exists in that round.
func (m *Mysticeti) findLeaderInRound(round uint32) *model.Event {
	leader := m.getRoundLeader(round)
	var leaderBlock *model.Event

	for _, head := range m.dag.GetHeads() {
		head.TraverseClosure(model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			if leaderBlock != nil {
				return model.Visit_Abort
			}

			eventRound := m.GetRound(e)

			if eventRound == 0 || eventRound < round {
				return model.Visit_Prune
			}
			if eventRound == round && e.Creator() == leader {
				leaderBlock = e
				return model.Visit_Abort
			}
			return model.Visit_Descent
		}))
		if leaderBlock != nil {
			break
		}
	}

	return leaderBlock
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
// - A block must include at least 2f+1 distinct hashes of blocks from the previous round
// - The first hash must be to the validator's own previous block (self-reference)
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
