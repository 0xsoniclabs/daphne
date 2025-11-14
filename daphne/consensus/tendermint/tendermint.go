package tendermint

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/emitter"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils/ruleset"
)

// Tendermint is a consensus protocol that is based on the paper herein
// https://arxiv.org/pdf/1807.04938. While the detailed overview is in the paper, the summary
// is as follows:
// - Consensus proceeds in heights, rounds and phases. Each height corresponds to a block
//   to be finalized. Each round is an attempt to finalize the block at the given height.
//   Each round consists of three phases: Propose, Prevote, Precommit.
// - In Propose phase, the leader proposes a block for the current height.
// - In Prevote phase, validators vote on the proposal, or a nil value, if they do not support
//   the current proposal. An honest validator will vote for the current proposal if it is equal
//   to the value it is locked on (or if it is not locked), or if it sees a proposal with a polka
//   that is newer than its locked round. Otherwise, it votes for nil.
// - In Precommit phase, validators precommit to the proposal if it received a quorum of prevotes,
//   or precommit to nil otherwise. At that point, it is also locked on that value. When a quorum
//   of precommits is observed for a value, the block is finalized.
// - This algorithm relies on timeouts for liveness.

const (
	DefaultPhaseTimeout = time.Millisecond * 500
)

// Factory is a factory for creating Tendermint consensus instances.
type Factory struct {
	// Timeouts for each phase. If zero, defaults will be used.
	ProposePhaseTimeout   time.Duration
	PrevotePhaseTimeout   time.Duration
	PrecommitPhaseTimeout time.Duration
	// PhaseTimeoutDelta is the increment to add to timeouts in each round.
	PhaseTimeoutDelta time.Duration
	// HeightLimit is the maximum height the consensus can reach. When this height is reached,
	// the consensus will stop. If zero, there is no limit.
	HeightLimit int
	// Committee is the committee used for consensus.
	Committee consensus.Committee
}

// Make a new passive Tendermint consensus instance. This instance does not propose blocks.
func (f *Factory) NewPassive(p2pServer p2p.Server) consensus.Consensus {
	return newTendermint(
		p2pServer,
		f.ProposePhaseTimeout,
		f.PrevotePhaseTimeout,
		f.PrecommitPhaseTimeout,
		f.PhaseTimeoutDelta,
		f.HeightLimit,
		f.Committee,
		0,
		false,
		nil,
	)
}

// Make a new active Tendermint consensus instance. This instance proposes blocks.
func (f *Factory) NewActive(p2pServer p2p.Server, selfId consensus.ValidatorId,
	source consensus.TransactionProvider) consensus.Consensus {
	return newTendermint(
		p2pServer,
		f.ProposePhaseTimeout,
		f.PrevotePhaseTimeout,
		f.PrecommitPhaseTimeout,
		f.PhaseTimeoutDelta,
		f.HeightLimit,
		f.Committee,
		selfId,
		true,
		source,
	)
}

type phase int

const (
	Propose phase = iota
	Prevote
	Precommit
)

// Tendermint is an implementation of the Tendermint consensus protocol.
type Tendermint struct {
	listeners      []consensus.BundleListener
	listenersMutex sync.Mutex

	// height is the current block height - consensus operates on heights independently.
	// This means that each time the height increases, everything else resets.
	height int
	// round is the current round within the height. Denotes the number of attempts to finalize
	// a block.
	round int
	// currentPhase is the current phase within the round.
	currentPhase phase
	// lockedValue and lockedRound as per Tendermint protocol. A proposal value is locked
	// when the validator observes a polka for it during Prevote phase. When a value is locked,
	// the validator can only vote for it or nil in subsequent rounds, unless it sees a proposal
	// with a newer polka.
	lockedValue *Block
	lockedRound int
	// latestPolkaValue and latestPolkaRound correspond to validValue/Round in Tendermint protocol.
	// They simply denote the latest proposal for which a polka was observed. Used to "unlock"
	// validators that are locked on older polkas.
	latestPolkaValue *Block
	latestPolkaRound int

	// phaseTimeouts specify the duration to wait in each phase before
	// triggering a timeout action.
	phaseTimeout map[phase]time.Duration
	// phaseTimeoutDelta specifies the increment to add to timeouts in each round.
	phaseTimeoutDelta time.Duration

	committee consensus.Committee
	selfId    consensus.ValidatorId

	stateMutex sync.Mutex
	// stopFlag indicates whether the consensus instance has been stopped.
	stopFlag bool
	// stopSignal is signaled when the consensus instance is stopped.
	stopSignal chan struct{}
	// nextRoundSignal is signaled when a new round starts.
	nextRoundSignal chan struct{}
	// heightLimit is the maximum height the consensus can reach. When this height is reached,
	// the consensus will stop.
	heightLimit int

	gossip   broadcast.Channel[Message]
	receiver broadcast.Receiver[Message]
	source   emitter.EmissionPayloadSource[Message]

	// isActive indicates whether this is an active consensus instance.
	isActive bool

	// messageLog tracks all received messages for applying rules.
	// They are tracked on a height basis.
	messageLog map[int][]Message

	// ruleset contains the Tendermint consensus rules.
	ruleset *ruleset.Ruleset[Message]

	// decidedForHeight tracks whether a decision has been made for a given height.
	decidedForHeight map[int]bool
}

func (t *Tendermint) RegisterListener(listener consensus.BundleListener) {
	t.listenersMutex.Lock()
	defer t.listenersMutex.Unlock()
	t.listeners = append(t.listeners, listener)
}

func (t *Tendermint) Stop() {
	t.stateMutex.Lock()
	defer t.stateMutex.Unlock()
	t.stop()
}

// The caller is assumed to hold the state mutex.
func (t *Tendermint) stop() {
	if t.stopFlag {
		return
	}
	t.stopFlag = true
	t.gossip.Unregister(t.receiver)
	close(t.stopSignal)
}

func newTendermint(
	p2pServer p2p.Server,
	proposePhaseTimeout time.Duration,
	prevotePhaseTimeout time.Duration,
	precommitPhaseTimeout time.Duration,
	phaseTimeoutDelta time.Duration,
	heightLimit int,
	committee consensus.Committee,
	selfId consensus.ValidatorId,
	isActive bool,
	source consensus.TransactionProvider,
) *Tendermint {
	if proposePhaseTimeout <= 0 {
		proposePhaseTimeout = DefaultPhaseTimeout
	}
	if prevotePhaseTimeout <= 0 {
		prevotePhaseTimeout = DefaultPhaseTimeout
	}
	if precommitPhaseTimeout <= 0 {
		precommitPhaseTimeout = DefaultPhaseTimeout
	}
	t := &Tendermint{
		phaseTimeout: map[phase]time.Duration{
			Propose:   proposePhaseTimeout,
			Prevote:   prevotePhaseTimeout,
			Precommit: precommitPhaseTimeout,
		},
		committee:         committee,
		height:            0,
		messageLog:        make(map[int][]Message),
		decidedForHeight:  make(map[int]bool),
		heightLimit:       heightLimit,
		phaseTimeoutDelta: phaseTimeoutDelta,
		isActive:          false,
	}
	t.stopSignal = make(chan struct{})
	t.nextRoundSignal = make(chan struct{})
	t.ruleset = getTendermintRuleset(t)
	t.receiver = broadcast.WrapReceiver(func(msg Message) {
		t.stateMutex.Lock()
		defer t.stateMutex.Unlock()
		if !t.stopFlag {
			t.handleMessage(msg)
		}
	})
	t.gossip = broadcast.NewGossip(
		p2pServer,
		func(msg Message) types.Hash {
			return msg.Hash()
		})
	t.gossip.Register(t.receiver)
	t.selfId = selfId
	t.source = emissionPayloadSourceAdapter{
		source:     source,
		tendermint: t,
	}
	t.isActive = isActive
	go func() {
		t.stateMutex.Lock()
		defer t.stateMutex.Unlock()
		t.reset()
		t.startRound(0)
	}()
	return t
}

// reset resets the Tendermint state for a new height.
func (t *Tendermint) reset() {
	t.lockedValue = nil
	t.lockedRound = -1
	t.latestPolkaValue = nil
	t.latestPolkaRound = -1
}

// startRound starts a new round.
// The caller is assumed to hold the state mutex.
// Lines 11-21 in the Tendermint paper pseudocode.
func (t *Tendermint) startRound(round int) {
	close(t.nextRoundSignal)
	t.nextRoundSignal = make(chan struct{})
	t.ruleset.Reset()
	t.round = round
	t.currentPhase = Propose
	if t.selfId == chooseLeader(t.height, t.round, t.committee) && t.isActive {
		msg := t.source.GetEmissionPayload()
		go t.gossip.Broadcast(msg)
	} else {
		stopSignal := t.stopSignal
		nextRoundSignal := t.nextRoundSignal
		go func() {
			select {
			case <-stopSignal:
			case <-nextRoundSignal:
				return
			case <-time.After(t.phaseTimeout[Propose] + t.phaseTimeoutDelta*time.Duration(round)):
				t.stateMutex.Lock()
				defer t.stateMutex.Unlock()
				t.onTimeoutPropose(t.height, t.round)
			}
		}()
	}
}

// handleMessage is called each time a message is received.
// The caller is assumed to hold the state mutex.
func (t *Tendermint) handleMessage(msg Message) {
	t.ruleset.Apply(msg)
}

// notifyListeners notifies all registered listeners of a new bundle.
func (t *Tendermint) notifyListeners(bundle types.Bundle) {
	t.listenersMutex.Lock()
	defer t.listenersMutex.Unlock()
	for _, listener := range t.listeners {
		listener.OnNewBundle(bundle)
	}
}

// predicateHasQuorum checks whether the given predicate has a quorum of messages.
// It is checked at a given height.
func (t *Tendermint) predicateHasQuorum(p func(Message) bool, height int) bool {
	counter := consensus.NewVoteCounter(&t.committee)
	list := t.getMessagesSatisfying(p, height)
	for _, msg := range list {
		counter.Vote(msg.Signature)
	}
	return counter.IsQuorumReached()
}

// predicateHasAtLeastOneHonestVote checks whether the given predicate has at least
// one honest vote. It is checked at a given height.
func (t *Tendermint) predicateHasAtLeastOneHonestVote(p func(Message) bool, height int) bool {
	counter := consensus.NewVoteCounter(&t.committee)
	list := t.getMessagesSatisfying(p, height)
	for _, msg := range list {
		counter.Vote(msg.Signature)
	}
	return counter.HasAtLeastOneHonestVote()
}

// getMessagesSatisfying returns all messages satisfying the given predicate.
func (t *Tendermint) getMessagesSatisfying(p func(Message) bool, height int) []Message {
	var result []Message
	list := t.messageLog[height]
	for _, msg := range list {
		if p(msg) {
			result = append(result, msg)
		}
	}
	return result
}

// getCurrentProposalMessage returns the current proposal message, if any.
func (t *Tendermint) getCurrentProposalMessage() *Message {
	proposals := t.getMessagesSatisfying(func(msg Message) bool {
		return msg.Phase == Propose && msg.Round == t.round &&
			msg.Signature == chooseLeader(t.height, t.round, t.committee)
	}, t.height)
	if len(proposals) > 0 {
		return &proposals[0]
	}
	return nil
}

// chooseLeader selects the leader for the given round based on round and height.
func chooseLeader(height int, round int, committee consensus.Committee) consensus.ValidatorId {
	validators := committee.Validators()
	return validators[(round+height)%len(validators)]
}

// --- TENDERMINT RULES ---

// isInPhase checks whether the current phase is the given phase.
func isInPhase(t *Tendermint, phase phase) func(Message) bool {
	return func(_ Message) bool {
		return t.currentPhase == phase
	}
}

// hasProposalWithPolkaRoundMinusOne checks whether there is a proposal with PolkaRound == -1.
func hasProposalWithPolkaRoundMinusOne(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		proposal := t.getCurrentProposalMessage()
		return proposal != nil && proposal.PolkaRound == -1
	}
}

// proposedBlockHasPolkaInItsEarlierPolkaRound checks whether the proposed block
// indeed had a polka in its PolkaRound (that has to be less than current round).
func proposedBlockHasPolkaInItsEarlierPolkaRound(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		proposal := t.getCurrentProposalMessage()
		if proposal == nil || proposal.PolkaRound >= t.round {
			return false
		}
		return t.predicateHasQuorum(func(msg Message) bool {
			return msg.Phase == Prevote &&
				msg.BlockId == proposal.Block.Id() &&
				msg.Round == proposal.PolkaRound
		}, t.height)
	}
}

// seenQuorumOfAnyPrevotes checks whether a quorum of validators has sent a prevote this round.
func seenQuorumOfAnyPrevotes(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		return t.predicateHasQuorum(func(msg Message) bool {
			return msg.Phase == Prevote && msg.Round == t.round
		}, t.height)
	}
}

// polkaOnProposal checks whether there is a polka on the current proposal.
func polkaOnProposal(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		proposal := t.getCurrentProposalMessage()
		if proposal == nil {
			return false
		}
		return t.predicateHasQuorum(func(msg Message) bool {
			return msg.Phase == Prevote &&
				msg.BlockId == proposal.Block.Id() &&
				msg.Round == proposal.Round
		}, t.height)
	}
}

// quorumOfNilPrevotes checks whether a quorum of validators has sent nil prevotes this round.
func quorumOfNilPrevotes(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		return t.predicateHasQuorum(func(msg Message) bool {
			return msg.Phase == Prevote &&
				msg.BlockId == types.Hash{} &&
				msg.Round == t.round
		}, t.height)
	}
}

// seenQuorumOfAnyPrecommits checks whether a quorum of validators has sent a precommit this round.
func seenQuorumOfAnyPrecommits(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		return t.predicateHasQuorum(func(msg Message) bool {
			return msg.Phase == Precommit && msg.Round == t.round
		}, t.height)
	}
}

// anyProposalHasQuorumOfPrecommits checks whether there is any proposal that has a quorum of precommits.
// If such a proposal is found, it is assigned to the given pointer.
func anyProposalHasQuorumOfPrecommits(t *Tendermint, p *Message) func(Message) bool {
	return func(Message) bool {
		allProposals := t.getMessagesSatisfying(func(msg Message) bool {
			return msg.Phase == Propose && msg.Signature == chooseLeader(t.height, msg.Round, t.committee)
		}, t.height)
		for _, proposal := range allProposals {
			hasQuorum := t.predicateHasQuorum(func(msg Message) bool {
				return msg.Phase == Precommit &&
					msg.BlockId == proposal.Block.Id() &&
					msg.Round == proposal.Round
			}, t.height)
			if hasQuorum {
				*p = proposal
				return true
			}
		}
		return false
	}
}

// atLeastOneHonestMessageFromLaterRound checks whether there is at least one honest message
// from a later round. If such a message is found, its round is assigned to the given pointer.
func atLeastOneHonestMessageFromLaterRound(t *Tendermint, round *int) func(Message) bool {
	return func(Message) bool {
		return t.predicateHasAtLeastOneHonestVote(func(msg Message) bool {
			*round = msg.Round
			return msg.Round > t.round
		}, t.height)
	}
}

// notDecidedThisHeight checks whether a decision has not been made for the current height.
func notDecidedThisHeight(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		return !t.decidedForHeight[t.height]
	}
}

// trackMessageRule is an unconditional rule that tracks all received messages.
func trackMessageRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetAction(func(msg Message) {
		if msg.Height >= t.height {
			t.messageLog[msg.Height] = append(t.messageLog[msg.Height], msg)
		}
	})
	return &rule
}

// freshProposalRule handles the case when a proposal without a PolkaRound is received.
// Lines 22-27 in the Tendermint paper pseudocode.
func freshProposalRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(
		ruleset.And(
			isInPhase(t, Propose),
			hasProposalWithPolkaRoundMinusOne(t),
		),
	)
	rule.SetAction(func(Message) {
		proposal := t.getCurrentProposalMessage()
		hash := types.Hash{}
		if t.lockedRound == -1 || t.lockedValue.Id() == proposal.Block.Id() {
			hash = proposal.Block.Id()
		}
		if t.isActive {
			msg := Message{
				Phase:     Prevote,
				Height:    t.height,
				Round:     t.round,
				BlockId:   hash,
				Signature: t.selfId,
			}
			go t.gossip.Broadcast(msg)
		}
		t.currentPhase = Prevote
	})
	return &rule
}

// polkaProposalRule handles the case when a proposal with a PolkaRound is received.
// Lines 28-33 in the Tendermint paper pseudocode.
func polkaProposalRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(
		ruleset.And(
			isInPhase(t, Propose),
			proposedBlockHasPolkaInItsEarlierPolkaRound(t),
		),
	)
	rule.SetAction(func(Message) {
		proposal := t.getCurrentProposalMessage()
		hash := types.Hash{}
		if t.lockedRound < proposal.PolkaRound ||
			t.lockedValue.Id() == proposal.Block.Id() {
			hash = proposal.Block.Id()
		}
		if t.isActive {
			msg := Message{
				Phase:     Prevote,
				Height:    t.height,
				Round:     t.round,
				BlockId:   hash,
				Signature: t.selfId,
			}
			go t.gossip.Broadcast(msg)
		}
		t.currentPhase = Prevote
	})
	return &rule
}

// timeoutPrevoteRule handles the timeout in Prevote phase.
// Lines 34-35 in the Tendermint paper pseudocode.
func timeoutPrevoteRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(
		ruleset.And(
			isInPhase(t, Prevote),
			seenQuorumOfAnyPrevotes(t),
		),
	)
	rule.SetAction(func(Message) {
		stopSignal := t.stopSignal
		nextRoundSignal := t.nextRoundSignal
		round := t.round
		go func() {
			select {
			case <-stopSignal:
			case <-nextRoundSignal:
				return
			case <-time.After(t.phaseTimeout[Prevote] + t.phaseTimeoutDelta*time.Duration(round)):
				t.stateMutex.Lock()
				defer t.stateMutex.Unlock()
				t.onTimeoutPrevote(t.height, t.round)
			}
		}()
	}).OnlyOnce()
	return &rule
}

// polkaObservedOnProposalRule handles the case when a polka is observed on the current proposal.
// Lines 36-43 in the Tendermint paper pseudocode.
func polkaObservedOnProposalRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(
		ruleset.And(
			ruleset.Not(isInPhase(t, Propose)),
			polkaOnProposal(t),
		),
	)
	rule.SetAction(func(Message) {
		proposal := t.getCurrentProposalMessage()
		if t.currentPhase == Prevote {
			t.lockedValue = proposal.Block
			t.lockedRound = proposal.Round
			if t.isActive {
				msg := Message{
					Phase:     Precommit,
					Height:    t.height,
					Round:     t.round,
					BlockId:   proposal.Block.Id(),
					Signature: t.selfId,
				}
				go t.gossip.Broadcast(msg)
			}
			t.currentPhase = Precommit
		}
		t.latestPolkaValue = proposal.Block
		t.latestPolkaRound = proposal.Round
	}).OnlyOnce()
	return &rule
}

// quorumOfNilPrevotesObservedRule handles the case when a quorum of nil prevotes is observed.
// Lines 44-46 in the Tendermint paper pseudocode.
func quorumOfNilPrevotesObservedRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(
		ruleset.And(
			isInPhase(t, Prevote),
			quorumOfNilPrevotes(t),
		),
	)
	rule.SetAction(func(Message) {
		if t.isActive {
			msg := Message{
				Phase:     Precommit,
				Height:    t.height,
				Round:     t.round,
				BlockId:   types.Hash{}, // nil vote
				Signature: t.selfId,
			}
			go t.gossip.Broadcast(msg)
		}
		t.currentPhase = Precommit
	})
	return &rule
}

// timeoutPrecommitRule handles the timeout in Precommit phase.
// Lines 47-48 in the Tendermint paper pseudocode.
func timeoutPrecommitRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(
		seenQuorumOfAnyPrecommits(t),
	)
	rule.SetAction(func(Message) {
		stopSignal := t.stopSignal
		nextRoundSignal := t.nextRoundSignal
		round := t.round
		go func() {
			select {
			case <-stopSignal:
			case <-nextRoundSignal:
				return
			case <-time.After(t.phaseTimeout[Precommit] + t.phaseTimeoutDelta*time.Duration(round)):
				t.stateMutex.Lock()
				defer t.stateMutex.Unlock()
				t.onTimeoutPrecommit(t.height, t.round)
			}
		}()
	}).OnlyOnce()
	return &rule
}

// decideRule handles the decision when a proposal has a quorum of precommits.
// This finalizes the block and moves to the next height.
// Lines 49-54 in the Tendermint paper pseudocode.
func decideRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	var proposal Message
	rule.SetCondition(
		ruleset.And(
			anyProposalHasQuorumOfPrecommits(t, &proposal),
			notDecidedThisHeight(t),
		),
	)
	rule.SetAction(func(Message) {
		t.decidedForHeight[t.height] = true
		t.notifyListeners(types.Bundle(*proposal.Block))
		t.height++
		if t.heightLimit > 0 && t.height >= t.heightLimit {
			t.stop()
			return
		}
		delete(t.messageLog, t.height-1)
		t.reset()
		t.startRound(0)
	})
	return &rule
}

// catchUpRule handles the case when at least one honest message from a later round is observed.
// Lines 55-56 in the Tendermint paper pseudocode.
func catchUpRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	var round int
	rule.SetCondition(atLeastOneHonestMessageFromLaterRound(t, &round))
	rule.SetAction(func(Message) {
		t.startRound(round)
	})
	return &rule
}

// getTendermintRuleset constructs the Tendermint ruleset.
// Shown are the line numbers from the Tendermint paper pseudocode for reference.
func getTendermintRuleset(t *Tendermint) *ruleset.Ruleset[Message] {
	ruleset := &ruleset.Ruleset[Message]{}
	ruleset.AddRule(
		trackMessageRule(t), 0)
	ruleset.AddRule(
		freshProposalRule(t), 1) // Lines 22-27.
	ruleset.AddRule(
		polkaProposalRule(t), 1) // Lines 28-33.
	ruleset.AddRule(
		timeoutPrevoteRule(t), 1) // Lines 34-35.
	ruleset.AddRule(
		polkaObservedOnProposalRule(t), 1) // Lines 36-43.
	ruleset.AddRule(
		quorumOfNilPrevotesObservedRule(t), 1) // Lines 44-46.
	ruleset.AddRule(
		timeoutPrecommitRule(t), 1) // Lines 47-48.
	ruleset.AddRule(
		decideRule(t), 1) // Lines 49-54.
	ruleset.AddRule(
		catchUpRule(t), 1) // Lines 55-56.
	return ruleset
}

// onTimeoutPropose handles the timeout in Propose phase. It moves to Prevote phase.
// The caller is assumed to hold the state mutex.
// Lines 57-60 in the Tendermint paper pseudocode.
func (t *Tendermint) onTimeoutPropose(height int, round int) {
	if t.height == height && t.round == round && t.currentPhase == Propose {
		t.currentPhase = Prevote
		if t.isActive {
			msg := Message{
				Phase:     Prevote,
				Height:    t.height,
				Round:     t.round,
				BlockId:   types.Hash{}, // nil vote
				Signature: t.selfId,
			}
			go t.gossip.Broadcast(msg)
		}
	}
}

// onTimeoutPropose handles the timeout in Prevote phase. It moves to Precommit phase.
// The caller is assumed to hold the state mutex.
// Lines 61-64 in the Tendermint paper pseudocode.
func (t *Tendermint) onTimeoutPrevote(height int, round int) {
	if t.height == height && t.round == round && t.currentPhase == Prevote {
		t.currentPhase = Precommit
		if t.isActive {
			msg := Message{
				Phase:     Precommit,
				Height:    t.height,
				Round:     t.round,
				BlockId:   types.Hash{}, // nil vote
				Signature: t.selfId,
			}
			go t.gossip.Broadcast(msg)
		}
	}
}

// onTimeoutPrecommit handles the timeout in Precommit phase. It starts a new round
// without finalizing a block.
// The caller is assumed to hold the state mutex.
// Lines 65-67 in the Tendermint paper pseudocode.
func (t *Tendermint) onTimeoutPrecommit(height int, round int) {
	if t.height == height && t.round == round && t.currentPhase == Precommit {
		t.startRound(t.round + 1)
	}
}

// --- HELPER TYPES ---

type emissionPayloadSourceAdapter struct {
	source     consensus.TransactionProvider
	tendermint *Tendermint
}

// GetEmissionPayload constructs the emission payload for the current round.
// Proposals are constructed via the TransactionProvider, chaining to the latest block.
func (a emissionPayloadSourceAdapter) GetEmissionPayload() Message {
	t := a.tendermint
	proposal := t.latestPolkaValue
	if proposal == nil {
		tx := a.source.GetCandidateTransactions()

		proposal = &Block{
			Number:       uint32(t.height),
			Transactions: tx,
		}
	}
	return Message{
		Phase:      Propose,
		Height:     t.height,
		Round:      t.round,
		Block:      proposal,
		PolkaRound: t.latestPolkaRound,
		Signature:  t.selfId,
	}
}

type Block types.Bundle

// Id generates a unique identifier for the block based on its contents.
func (b Block) Id() types.Hash {
	data := fmt.Sprintf("%+v", b)
	return types.Sha256([]byte(data))
}

type Message struct {
	// Signature is an unforgeable identifier of the validator.
	Signature consensus.ValidatorId
	// Phase indicates the consensus phase of this message.
	Phase phase
	// Block and BlockId are mutually exclusive. Block is used for proposals,
	// while BlockId is used for votes (prevotes and precommits). This is to avoid
	// resending the full block with every vote message.
	// Empty hash means nil block.
	Block   *Block
	BlockId types.Hash
	// Height is the height of the block this message is about.
	Height int
	// Round is the current consensus round.
	Round int
	// PolkaRound is the latest round in which a polka was observed.
	PolkaRound int
}

func (m Message) Hash() types.Hash {
	data := fmt.Sprintf("%+v", m)
	return types.Sha256([]byte(data))
}

func (m Message) GetMessageType() p2p.MessageType {
	return p2p.MessageType("Tendermint")
}

func (m Message) MessageSize() uint32 {
	res := uint32(reflect.TypeFor[Message]().Size())
	if m.Block != nil {
		res += uint32(reflect.TypeFor[Block]().Size())
		for _, tx := range m.Block.Transactions {
			res += tx.MessageSize()
		}
	}
	return res
}
