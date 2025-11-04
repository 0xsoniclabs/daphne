package tendermint

import (
	"fmt"
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/emitter"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils/ruleset"
)

const (
	DefaultPhaseTimeout = time.Millisecond * 500
)

type Factory struct {
	ProposePhaseTimeout   time.Duration
	PrevotePhaseTimeout   time.Duration
	PrecommitPhaseTimeout time.Duration
	PhaseTimeoutDelta     time.Duration
	HeightLimit           int
	Committee             consensus.Committee
}

func (f *Factory) NewPassive(p2pServer p2p.Server) consensus.Consensus {
	return newPassiveTendermint(
		p2pServer,
		f.ProposePhaseTimeout,
		f.PrevotePhaseTimeout,
		f.PrecommitPhaseTimeout,
		f.PhaseTimeoutDelta,
		f.HeightLimit,
		f.Committee,
	)
}

func (f *Factory) NewActive(p2pServer p2p.Server, selfId consensus.ValidatorId,
	source consensus.TransactionProvider) consensus.Consensus {
	return newActiveTendermint(
		p2pServer,
		source,
		f.ProposePhaseTimeout,
		f.PrevotePhaseTimeout,
		f.PrecommitPhaseTimeout,
		f.PhaseTimeoutDelta,
		f.HeightLimit,
		f.Committee,
		selfId,
	)
}

type phase int

const (
	Propose phase = iota
	Prevote
	Precommit
)

type Tendermint struct {
	listeners      []consensus.BundleListener
	listenersMutex sync.Mutex

	lastBlockHash types.Hash

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
	// roundCounter is incremented each time a new round starts. It is used
	// to determine the leader for each round - to ensure fairness.
	roundCounter int

	stateMutex sync.Mutex
	// stopFlag indicates whether the consensus instance has been stopped.
	stopFlag bool
	// stopSignal is signaled when the consensus instance is stopped.
	stopSignal chan struct{}
	// heightLimit is the maximum height the consensus can reach. When this height is reached,
	// the consensus will stop.
	heightLimit int

	gossip   broadcast.Channel[Message]
	receiver broadcast.Receiver[Message]
	source   emitter.EmissionPayloadSource[Message]

	// currentProposal is the proposal that is currently being voted on.
	currentProposal *Block
	// Trackers memorize votes for different message patterns to determine
	// when quorums are reached.
	wholeMessageTracker                quorumTracker
	withoutBlockIdAndPolkaRoundTracker quorumTracker
	onlyHeightAndRoundTracker          quorumTracker

	// ruleset contains the Tendermint consensus rules.
	ruleset *ruleset.Ruleset[Message]

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
	t.stopFlag = true
	t.gossip.Unregister(t.receiver)
	close(t.stopSignal)
}

func newPassiveTendermint(
	p2pServer p2p.Server,
	proposePhaseTimeout time.Duration,
	prevotePhaseTimeout time.Duration,
	precommitPhaseTimeout time.Duration,
	phaseTimeoutDelta time.Duration,
	heightLimit int,
	committee consensus.Committee,
) *Tendermint {
	if proposePhaseTimeout == 0 {
		proposePhaseTimeout = DefaultPhaseTimeout
	}
	if prevotePhaseTimeout == 0 {
		prevotePhaseTimeout = DefaultPhaseTimeout
	}
	if precommitPhaseTimeout == 0 {
		precommitPhaseTimeout = DefaultPhaseTimeout
	}
	t := &Tendermint{
		phaseTimeout: map[phase]time.Duration{
			Propose:   proposePhaseTimeout,
			Prevote:   prevotePhaseTimeout,
			Precommit: precommitPhaseTimeout,
		},
		committee:        committee,
		round:            0,
		roundCounter:     0,
		height:           0,
		lockedRound:      -1,
		latestPolkaRound: -1,
		wholeMessageTracker: quorumTracker{
			counters:  make(map[MessagePattern]*consensus.VoteCounter),
			committee: committee,
		},
		withoutBlockIdAndPolkaRoundTracker: quorumTracker{
			counters:  make(map[MessagePattern]*consensus.VoteCounter),
			committee: committee,
		},
		onlyHeightAndRoundTracker: quorumTracker{
			counters:  make(map[MessagePattern]*consensus.VoteCounter),
			committee: committee,
		},
		decidedForHeight:  make(map[int]bool),
		heightLimit:       heightLimit,
		phaseTimeoutDelta: phaseTimeoutDelta,
	}
	t.stopSignal = make(chan struct{})
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
	return t
}

func newActiveTendermint(
	p2pServer p2p.Server,
	source consensus.TransactionProvider,
	proposePhaseTimeout time.Duration,
	prevotePhaseTimeout time.Duration,
	precommitPhaseTimeout time.Duration,
	phaseTimeoutDelta time.Duration,
	heightLimit int,
	committee consensus.Committee,
	selfId consensus.ValidatorId,
) *Tendermint {
	t := newPassiveTendermint(
		p2pServer,
		proposePhaseTimeout,
		prevotePhaseTimeout,
		precommitPhaseTimeout,
		phaseTimeoutDelta,
		heightLimit,
		committee,
	)
	t.selfId = selfId
	t.source = emissionPayloadSourceAdapter{
		source:     source,
		tendermint: t,
	}
	go func() {
		t.stateMutex.Lock()
		defer t.stateMutex.Unlock()
		t.startRound(0)
	}()
	return t
}

// The caller is assumed to hold the state mutex.
func (t *Tendermint) startRound(round int) {
	if t.stopFlag {
		return
	}
	t.round = round
	t.currentPhase = Propose
	t.currentProposal = nil
	if t.selfId == chooseLeader(t.roundCounter, t.committee) {
		msg := t.source.GetEmissionPayload()
		go t.gossip.Broadcast(msg)
	} else {
		go func() {
			select {
			case <-t.stopSignal:
				return
			case <-time.After(t.phaseTimeout[Propose] + t.phaseTimeoutDelta*time.Duration(t.round)):
				t.stateMutex.Lock()
				defer t.stateMutex.Unlock()
				t.onTimeoutPropose(t.height, t.round)
			}
		}()
	}
}

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

// chooseLeader selects the leader for the given round based on a simple
// round-robin approach.
func chooseLeader(counter int, committee consensus.Committee) consensus.ValidatorId {
	validators := committee.Validators()
	return validators[counter%len(validators)]
}

// --- TENDERMINT RULES ---

func isInPhase(t *Tendermint, phase phase) ruleset.Condition[Message] {
	return func(msg Message) bool {
		return t.currentPhase == phase
	}
}

func isMessageInPhase(phase phase) ruleset.Condition[Message] {
	return func(msg Message) bool {
		return msg.Phase == phase
	}
}

func isMessageFromLeader(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		leader := chooseLeader(t.roundCounter, t.committee)
		return msg.Signature == leader
	}
}

func heightRoundMatch(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		return msg.Height == t.height && msg.Round == t.round
	}
}

func blockNotNil() ruleset.Condition[Message] {
	return func(msg Message) bool {
		return msg.Block != nil
	}
}

func polkaRoundMinusOne() ruleset.Condition[Message] {
	return func(msg Message) bool {
		return msg.PolkaRound == -1
	}
}

func hasEarlierPolkaRound(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		return msg.PolkaRound < t.round
	}
}

func messageHasPolka(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		forQuorumCheck := MessagePattern{
			Phase:   Prevote,
			Height:  msg.Height,
			Round:   msg.PolkaRound,
			BlockId: msg.Block.Id(),
		}
		return t.wholeMessageTracker.isQuorumReached(forQuorumCheck)
	}
}

func proposalHadPolka(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		if t.currentProposal == nil {
			return false
		}
		forQuorumCheck := MessagePattern{
			Phase:   Prevote,
			Height:  t.height,
			Round:   t.round,
			BlockId: t.currentProposal.Id(),
		}
		return t.wholeMessageTracker.isQuorumReached(forQuorumCheck)
	}
}

func proposalHasQuorumPrecommits(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		if t.currentProposal == nil {
			return false
		}
		pattern := MessagePattern{
			Phase:   Precommit,
			Height:  t.height,
			Round:   t.latestPolkaRound,
			BlockId: t.currentProposal.Id(),
		}
		return t.wholeMessageTracker.isQuorumReached(pattern)
	}
}

func hasQuorumPrevotesThisRound(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		pattern := MessagePattern{
			Phase:  Prevote,
			Height: t.height,
			Round:  t.round,
		}
		return t.withoutBlockIdAndPolkaRoundTracker.isQuorumReached(pattern)
	}
}

func hasQuorumNilPrevotesThisRound(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		pattern := MessagePattern{
			Phase:   Prevote,
			Height:  t.height,
			Round:   t.round,
			BlockId: types.Hash{},
		}
		return t.wholeMessageTracker.isQuorumReached(pattern)
	}
}

func hasQuorumPrecommitsThisRound(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		pattern := MessagePattern{
			Phase:  Precommit,
			Height: t.height,
			Round:  t.round,
		}
		return t.withoutBlockIdAndPolkaRoundTracker.isQuorumReached(pattern)
	}
}

func hasDecidedForHeight(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		return t.decidedForHeight[t.height]
	}
}

func atLeastOneHonestMessageFromTheRoundOfIncomingMessage(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		pattern := MessagePattern{
			Height: t.height,
			Round:  msg.Round,
		}
		return t.onlyHeightAndRoundTracker.hasAtLeastOneHonestVote(pattern)
	}
}

func incomingMessageRoundHigherThanCurrentRound(t *Tendermint) ruleset.Condition[Message] {
	return func(msg Message) bool {
		return msg.Round > t.round
	}
}

func trackMessageRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := &ruleset.Rule[Message]{}
	rule.SetAction(func(msg Message) {
		signature := msg.Signature

		fullMessagePattern := MessagePattern(msg)
		fullMessagePattern.Signature = consensus.ValidatorId(0)
		t.wholeMessageTracker.vote(fullMessagePattern, signature)

		withoutBlockIdAndPolkaRound := fullMessagePattern
		withoutBlockIdAndPolkaRound.BlockId = types.Hash{}
		withoutBlockIdAndPolkaRound.PolkaRound = 0
		t.withoutBlockIdAndPolkaRoundTracker.vote(withoutBlockIdAndPolkaRound, signature)

		withOnlyHeightAndRound := MessagePattern{Height: fullMessagePattern.Height, Round: fullMessagePattern.Round}
		t.onlyHeightAndRoundTracker.vote(withOnlyHeightAndRound, signature)
	})
	return rule
}

func freshProposalRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := &ruleset.Rule[Message]{}
	cond := ruleset.And(
		isMessageFromLeader(t),
		isMessageInPhase(Propose),
		isInPhase(t, Propose),
		heightRoundMatch(t),
		blockNotNil(),
		polkaRoundMinusOne(),
	)
	rule.SetCondition(cond)
	rule.SetAction(func(msg Message) {
		var vote types.Hash = types.Hash{}
		if msg.Block != nil &&
			(t.lockedRound == -1 || t.lockedValue.Id() == msg.Block.Id()) {
			vote = msg.Block.Id()
		}
		newMsg := Message{
			Phase:     Prevote,
			Height:    t.height,
			Round:     t.round,
			BlockId:   vote,
			Signature: t.selfId,
		}
		go t.gossip.Broadcast(newMsg)
		t.currentProposal = msg.Block
		t.currentPhase = Prevote
	})
	return rule
}

func polkaProposalRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := &ruleset.Rule[Message]{}
	cond := ruleset.And(
		isMessageFromLeader(t),
		isMessageInPhase(Propose),
		isInPhase(t, Propose),
		ruleset.Not(polkaRoundMinusOne()),
		hasEarlierPolkaRound(t),
		heightRoundMatch(t),
		blockNotNil(),
		messageHasPolka(t),
	)
	rule.SetCondition(cond)
	rule.SetAction(func(msg Message) {
		var vote types.Hash = types.Hash{}
		if msg.Block != nil &&
			(t.lockedRound <= msg.PolkaRound ||
				t.lockedValue.Id() == msg.Block.Id()) {
			vote = msg.Block.Id()
		}
		newMsg := Message{
			Phase:     Prevote,
			Height:    t.height,
			Round:     t.round,
			BlockId:   vote,
			Signature: t.selfId,
		}
		go t.gossip.Broadcast(newMsg)
		t.currentProposal = msg.Block
		t.currentPhase = Prevote
	})
	return rule
}

func timeoutPrevoteRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := &ruleset.Rule[Message]{}
	cond := ruleset.And(
		isInPhase(t, Prevote),
		hasQuorumPrevotesThisRound(t),
	)
	rule.SetCondition(cond)
	rule.SetAction(func(msg Message) {
		go func() {
			select {
			case <-t.stopSignal:
				return
			case <-time.After(t.phaseTimeout[Prevote] + t.phaseTimeoutDelta*time.Duration(t.round)):
				t.stateMutex.Lock()
				defer t.stateMutex.Unlock()
				t.onTimeoutPrevote(t.height, t.round)
			}
		}()
	}).OnlyOnce()
	return rule
}

func observePolkaOnProposalRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := &ruleset.Rule[Message]{}
	cond := ruleset.And(
		ruleset.Not(isInPhase(t, Propose)),
		proposalHadPolka(t),
	)
	rule.SetCondition(cond)
	rule.SetAction(func(msg Message) {
		if t.currentPhase == Prevote {
			t.lockedValue = t.currentProposal
			t.lockedRound = t.round
			newMsg := Message{
				Phase:     Precommit,
				Height:    t.height,
				Round:     t.round,
				BlockId:   t.currentProposal.Id(),
				Signature: t.selfId,
			}
			go t.gossip.Broadcast(newMsg)
			t.currentPhase = Precommit
		}
		t.latestPolkaValue = t.currentProposal
		t.latestPolkaRound = t.round
	}).OnlyOnce()
	return rule
}

func observeNilPrevoteQuorumRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := &ruleset.Rule[Message]{}
	cond := ruleset.And(
		isInPhase(t, Prevote),
		hasQuorumNilPrevotesThisRound(t),
	)
	rule.SetCondition(cond)
	rule.SetAction(func(msg Message) {
		newMsg := Message{
			Phase:     Precommit,
			Height:    t.height,
			Round:     t.round,
			BlockId:   types.Hash{},
			Signature: t.selfId,
		}
		go t.gossip.Broadcast(newMsg)
		t.currentPhase = Precommit
	})
	return rule
}

func timeoutPrecommitRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := &ruleset.Rule[Message]{}
	cond := ruleset.And(
		hasQuorumPrecommitsThisRound(t),
	)
	rule.SetCondition(cond)
	rule.SetAction(func(msg Message) {
		go func() {
			select {
			case <-t.stopSignal:
				return
			case <-time.After(t.phaseTimeout[Precommit] + t.phaseTimeoutDelta*time.Duration(t.round)):
				t.stateMutex.Lock()
				defer t.stateMutex.Unlock()
				t.onTimeoutPrecommit(t.height, t.round)
			}
		}()
	}).OnlyOnce()
	return rule
}

func decideRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := &ruleset.Rule[Message]{}
	cond := ruleset.And(
		ruleset.Not(hasDecidedForHeight(t)),
		proposalHasQuorumPrecommits(t),
	)
	rule.SetCondition(cond)
	rule.SetAction(func(msg Message) {
		t.decidedForHeight[t.height] = true

		t.lockedValue = nil
		t.lockedRound = -1
		t.latestPolkaValue = nil
		t.latestPolkaRound = -1
		t.ruleset.Reset()

		newBundle := types.Bundle{
			Number:       uint32(t.height),
			Transactions: t.currentProposal.Transactions,
		}
		t.notifyListeners(newBundle)
		t.height++
		if t.height == t.heightLimit {
			t.stop()
		}
		t.roundCounter++
		t.startRound(0)
	})
	return rule
}

func catchUpRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := &ruleset.Rule[Message]{}
	cond := ruleset.And(
		atLeastOneHonestMessageFromTheRoundOfIncomingMessage(t),
		incomingMessageRoundHigherThanCurrentRound(t),
	)
	rule.SetCondition(cond)
	rule.SetAction(func(msg Message) {
		t.ruleset.Reset()
		t.startRound(msg.Round)
	})
	return rule
}

func getTendermintRuleset(t *Tendermint) *ruleset.Ruleset[Message] {
	ruleset := &ruleset.Ruleset[Message]{}
	ruleset.AddRule(
		trackMessageRule(t), 0)
	ruleset.AddRule(
		freshProposalRule(t), 1)
	ruleset.AddRule(
		polkaProposalRule(t), 1)
	ruleset.AddRule(
		timeoutPrevoteRule(t), 1)
	ruleset.AddRule(
		observePolkaOnProposalRule(t), 1)
	ruleset.AddRule(
		observeNilPrevoteQuorumRule(t), 1)
	ruleset.AddRule(
		timeoutPrecommitRule(t), 1)
	ruleset.AddRule(
		decideRule(t), 1)
	ruleset.AddRule(
		catchUpRule(t), 1)
	return ruleset
}

// The caller is assumed to hold the state mutex.
func (t *Tendermint) onTimeoutPropose(height int, round int) {
	if t.height == height && t.round == round && t.currentPhase == Propose {
		t.currentPhase = Prevote
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

// The caller is assumed to hold the state mutex.
func (t *Tendermint) onTimeoutPrevote(height int, round int) {
	if t.height == height && t.round == round && t.currentPhase == Prevote {
		t.currentPhase = Precommit
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

// The caller is assumed to hold the state mutex.
func (t *Tendermint) onTimeoutPrecommit(height int, round int) {
	if t.height == height && t.round == round && t.currentPhase == Precommit {
		t.ruleset.Reset()
		t.roundCounter++
		t.startRound(t.round + 1)
	}
}

// --- HELPER TYPES ---

type emissionPayloadSourceAdapter struct {
	source     consensus.TransactionProvider
	tendermint *Tendermint
}

func (a emissionPayloadSourceAdapter) GetEmissionPayload() Message {
	t := a.tendermint
	proposal := t.latestPolkaValue
	if proposal == nil {
		var tx []types.Transaction
		tx = a.source.GetCandidateTransactions()

		proposal = &Block{
			LastBlockHash: t.lastBlockHash,
			Number:        uint32(t.height),
			Transactions:  tx,
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

// quorumTracker tracks votes for different message patterns to determine
// when quorums are reached.
type quorumTracker struct {
	counters  map[MessagePattern]*consensus.VoteCounter
	committee consensus.Committee
}

func (qt quorumTracker) vote(pattern MessagePattern, voter consensus.ValidatorId) {
	ctr := qt.counters
	if _, exists := ctr[pattern]; !exists {
		ctr[pattern] = consensus.NewVoteCounter(&qt.committee)
	}
	ctr[pattern].Vote(voter)
}

func (qt quorumTracker) isQuorumReached(pattern MessagePattern) bool {
	if counter, exists := qt.counters[pattern]; exists {
		return counter.IsQuorumReached()
	}
	return false
}

func (qt quorumTracker) hasAtLeastOneHonestVote(pattern MessagePattern) bool {
	if counter, exists := qt.counters[pattern]; exists {
		return counter.HasAtLeastOneHonestVote()
	}
	return false
}

type Block struct {
	LastBlockHash types.Hash
	Number        uint32
	Transactions  []types.Transaction
}

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

type MessagePattern Message
