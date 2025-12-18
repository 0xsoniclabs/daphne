package emitter

import (
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

// MysticetiRoundTracker tracks per-validator state for Mysticeti consensus.
// Each validator independently tracks when it can disseminate blocks.
//
// From paper Section II-C (Liveness intuition):
// "We deem this block as the primary block of the round r and require that validators
// at r+1 wait a timeout for it to arrive before disseminating their blocks.
// Additionally, if the block is in the view of a validator at r+1 we further require
// the validator to wait another timeout for r+2 or until there are 2f+1 votes for
// the primary block of r."
//
// Block dissemination rules (Section II-C):
//  1. Self-reference: Every block must reference the validator's own prior block
//  2. DAG connectivity: Have 2f+1 blocks from round r-1
//  3. Primary wait: At round r, wait for primary block of r-1 to arrive OR Δ timeout
//  4. Certification wait: If primary of r-1 is in view, wait for 2f+1 blocks
//     referencing it (certification) OR another Δ timeout before disseminating
type MysticetiRoundTracker struct {
	committee *consensus.Committee

	// validatorId is the local validator ID
	validatorId consensus.ValidatorId

	// delta is the timeout duration (Δ in the paper)
	delta time.Duration

	// currentRound is the round this validator can currently propose for
	currentRound uint32

	// roundEntryTime tracks when we became eligible to enter each round
	// map[round]time.Time
	roundEntryTime map[uint32]time.Time

	// primarySeenTime tracks when we first saw the primary block for each round
	// map[round]time.Time - zero time if not seen
	primarySeenTime map[uint32]time.Time

	// eventsPerRound tracks validators that have events in each round
	// map[round]map[validatorId]*model.Event
	eventsPerRound map[uint32]map[consensus.ValidatorId]*model.Event

	// ownEventPerRound tracks our own event for each round (nil if not created yet)
	// map[round]*model.Event
	ownEventPerRound map[uint32]*model.Event

	// getLeaderFunc returns the primary (leader) validator for a given round
	getLeaderFunc func(round uint32) consensus.ValidatorId

	// getRoundFunc returns the round for an event (delegates to Mysticeti)
	getRoundFunc func(event *model.Event) uint32

	// supportsFunc checks if voter supports candidate (delegates to Mysticeti for proper DFS semantics)
	supportsFunc func(voter, candidate *model.Event) bool

	mu sync.RWMutex
}

// NewMysticetiRoundTracker creates a new state-based round tracker for Mysticeti.
func NewMysticetiRoundTracker(
	committee *consensus.Committee,
	validatorId consensus.ValidatorId,
	delta time.Duration,
	getLeaderFunc func(round uint32) consensus.ValidatorId,
	getRoundFunc func(event *model.Event) uint32,
	supportsFunc func(voter, candidate *model.Event) bool,
) *MysticetiRoundTracker {
	rt := &MysticetiRoundTracker{
		committee:        committee,
		validatorId:      validatorId,
		delta:            delta,
		currentRound:     1, // Start at round 1
		roundEntryTime:   make(map[uint32]time.Time),
		primarySeenTime:  make(map[uint32]time.Time),
		eventsPerRound:   make(map[uint32]map[consensus.ValidatorId]*model.Event),
		ownEventPerRound: make(map[uint32]*model.Event),
		getLeaderFunc:    getLeaderFunc,
		getRoundFunc:     getRoundFunc,
		supportsFunc:     supportsFunc,
	}
	// Mark entry time for round 1 (bootstrap)
	rt.roundEntryTime[1] = time.Now()
	return rt
}

// CurrentRound returns the round this validator can currently propose for.
func (rt *MysticetiRoundTracker) CurrentRound() uint32 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.currentRound
}

// CanProposeBlock checks if the validator can disseminate a block for the current round.
// Returns (canDisseminate, round)
//
// From paper Section II-C, block dissemination conditions:
//  1. Haven't already disseminated for round r
//  2. Self-reference: Must have own block from a prior round (first parent must be self)
//  3. DAG connectivity: Have 2f+1 blocks from round r-1
//  4. Primary wait: Wait for primary block of r-1 to arrive OR Δ timeout since entering round
//  5. Certification wait: If primary of r-1 is in view, wait for 2f+1 blocks in r
//     that support it (certification) OR another Δ timeout
func (rt *MysticetiRoundTracker) CanProposeBlock() (bool, uint32) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	round := rt.currentRound

	// Condition 1: Check if we already proposed for this round
	if rt.ownEventPerRound[round] != nil {
		return false, round
	}

	quorum := rt.committee.Quorum()

	// Condition 2: For round >= 2, must have own block from prior round (self-reference)
	if round >= 2 {
		hasOwnPriorBlock := false
		for r := uint32(1); r < round; r++ {
			if rt.ownEventPerRound[r] != nil {
				hasOwnPriorBlock = true
				break
			}
		}
		if !hasOwnPriorBlock {
			return false, round
		}
	}

	// For round 1, we can propose immediately if conditions 1-2 are met
	if round == 1 {
		return true, round
	}

	// Condition 3: For round >= 2, must have 2f+1 blocks from r-1 (DAG connectivity)
	if round >= 2 {
		prevRoundCount := uint32(len(rt.eventsPerRound[round-1]))
		if prevRoundCount < quorum {
			return false, round
		}
	}

	// Conditions 4 & 5: Primary block and timeout conditions
	// These apply when proposing for round r, waiting on primary from round r-1
	prevRound := round - 1
	primary := rt.getLeaderFunc(prevRound)
	primaryEvent := rt.eventsPerRound[prevRound][primary]

	entryTime, hasEntryTime := rt.roundEntryTime[round]
	if !hasEntryTime {
		// Set entry time now
		rt.roundEntryTime[round] = time.Now()
		entryTime = rt.roundEntryTime[round]
	}

	if primaryEvent == nil {
		// Condition 4a: Primary NOT in view - wait for delta timeout since entering round
		deltaElapsed := time.Since(entryTime) >= rt.delta
		return deltaElapsed, round
	}

	// Condition 4b/5: Primary IS in view
	primarySeenTime := rt.primarySeenTime[prevRound]
	if primarySeenTime.IsZero() {
		// Just saw primary for first time, record it
		rt.primarySeenTime[prevRound] = time.Now()
		primarySeenTime = rt.primarySeenTime[prevRound]
	}

	// Check if delta timeout has passed since seeing primary
	deltaElapsedSincePrimary := time.Since(primarySeenTime) >= rt.delta
	if deltaElapsedSincePrimary {
		return true, round
	}

	// Check if we have 2f+1 blocks in current round that support the primary (certification)
	// This is the certification for the previous round's primary
	certificationCount := rt.countBlocksSupportingPrimary(primaryEvent, round)
	if certificationCount >= quorum {
		return true, round
	}

	return false, round
}

// countBlocksSupportingPrimary counts how many blocks in targetRound support the primary.
// Uses the proper "support" semantics from the paper (DFS first-encounter).
// Must be called with lock held.
func (rt *MysticetiRoundTracker) countBlocksSupportingPrimary(primaryEvent *model.Event, targetRound uint32) uint32 {
	if primaryEvent == nil {
		return 0
	}

	count := uint32(0)

	for _, event := range rt.eventsPerRound[targetRound] {
		if event == nil {
			continue
		}
		// Use the paper's support semantics (DFS first-encounter)
		if rt.supportsFunc != nil && rt.supportsFunc(event, primaryEvent) {
			count++
		}
	}

	return count
}

// OnEventCreated should be called when the local validator creates a new event.
// This registers the event and may trigger round advancement.
func (rt *MysticetiRoundTracker) OnEventCreated(event *model.Event, round uint32) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.ownEventPerRound[round] = event
	rt.recordEventForRoundLocked(event, round)

	// Check if this was the primary's event - record time seen
	if rt.getLeaderFunc(round) == event.Creator() {
		if rt.primarySeenTime[round].IsZero() {
			rt.primarySeenTime[round] = time.Now()
		}
	}

	// Try to advance to next round
	rt.tryAdvanceRoundLocked()
}

// OnEventReceived should be called when an event is received from the network.
func (rt *MysticetiRoundTracker) OnEventReceived(event *model.Event, round uint32) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.recordEventForRoundLocked(event, round)

	// Check if this is the primary's event for this round - record time seen
	if rt.getLeaderFunc(round) == event.Creator() {
		if rt.primarySeenTime[round].IsZero() {
			rt.primarySeenTime[round] = time.Now()
		}
	}

	// Try to advance to next round
	rt.tryAdvanceRoundLocked()
}

// tryAdvanceRoundLocked checks if conditions are met to advance to the next round.
// Must be called with lock held.
//
// From paper: A validator advances to round r+1 when it has:
// 1. 2f+1 blocks from round r
// 2. Already proposed a block for round r
func (rt *MysticetiRoundTracker) tryAdvanceRoundLocked() {
	quorum := rt.committee.Quorum()

	for {
		currentRoundCount := uint32(len(rt.eventsPerRound[rt.currentRound]))
		if currentRoundCount < quorum {
			// Don't have quorum for current round yet
			return
		}

		// Check if we can enter the next round
		nextRound := rt.currentRound + 1

		// Mark entry time for next round if not already set
		if _, exists := rt.roundEntryTime[nextRound]; !exists {
			rt.roundEntryTime[nextRound] = time.Now()
		}

		// Only advance currentRound if we've already proposed for it
		if rt.ownEventPerRound[rt.currentRound] != nil {
			rt.currentRound = nextRound
		} else {
			// Haven't proposed for current round yet, stay here
			return
		}
	}
}

// recordEventForRoundLocked records that a validator has an event in a given round.
// Must be called with lock held.
func (rt *MysticetiRoundTracker) recordEventForRoundLocked(event *model.Event, round uint32) {
	if rt.eventsPerRound[round] == nil {
		rt.eventsPerRound[round] = make(map[consensus.ValidatorId]*model.Event)
	}
	rt.eventsPerRound[round][event.Creator()] = event
}
