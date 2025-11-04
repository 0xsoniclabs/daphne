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

	// messageLog tracks all received messages for applying rules.
	// They are tracked on a height basis.
	messageLog map[int][]Message

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
		committee:         committee,
		round:             0,
		height:            0,
		lockedRound:       -1,
		latestPolkaRound:  -1,
		messageLog:        make(map[int][]Message),
		decidedForHeight:  make(map[int]bool),
		heightLimit:       heightLimit,
		phaseTimeoutDelta: phaseTimeoutDelta,
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
	close(t.nextRoundSignal)
	t.nextRoundSignal = make(chan struct{})
	t.ruleset.Reset()
	t.round = round
	t.currentPhase = Propose
	if t.selfId == chooseLeader(t.height, t.round, t.committee) {
		msg := t.source.GetEmissionPayload()
		go t.gossip.Broadcast(msg)
	} else {
		go func() {
			select {
			case <-t.stopSignal:
			case <-t.nextRoundSignal:
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

func isInPhase(t *Tendermint, phase phase) func(Message) bool {
	return func(_ Message) bool {
		return t.currentPhase == phase
	}
}

func hasProposalWithPolkaRoundMinusOne(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		proposal := t.getCurrentProposalMessage()
		return proposal != nil && proposal.PolkaRound == -1
	}
}

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

func seenQuorumOfAnyPrevotes(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		return t.predicateHasQuorum(func(msg Message) bool {
			return msg.Phase == Prevote && msg.Round == t.round
		}, t.height)
	}
}

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

func quorumOfNilPrevotes(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		return t.predicateHasQuorum(func(msg Message) bool {
			return msg.Phase == Prevote &&
				msg.BlockId == types.Hash{} &&
				msg.Round == t.round
		}, t.height)
	}
}

func seenQuorumOfAnyPrecommits(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		return t.predicateHasQuorum(func(msg Message) bool {
			return msg.Phase == Precommit && msg.Round == t.round
		}, t.height)
	}
}

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

func atLeastOneHonestMessageFromLaterRound(t *Tendermint, round *int) func(Message) bool {
	return func(Message) bool {
		return t.predicateHasAtLeastOneHonestVote(func(msg Message) bool {
			*round = msg.Round
			return msg.Round > t.round
		}, t.height)
	}
}

func notDecidedThisHeight(t *Tendermint) func(Message) bool {
	return func(Message) bool {
		return !t.decidedForHeight[t.height]
	}
}

func trackMessageRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetAction(func(msg Message) {
		t.messageLog[msg.Height] = append(t.messageLog[msg.Height], msg)
	})
	return &rule
}

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
		msg := Message{
			Phase:     Prevote,
			Height:    t.height,
			Round:     t.round,
			BlockId:   hash,
			Signature: t.selfId,
		}
		go t.gossip.Broadcast(msg)
		t.currentPhase = Prevote
	})
	return &rule
}

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
		msg := Message{
			Phase:     Prevote,
			Height:    t.height,
			Round:     t.round,
			BlockId:   hash,
			Signature: t.selfId,
		}
		go t.gossip.Broadcast(msg)
		t.currentPhase = Prevote
	})
	return &rule
}

func timeoutPrevoteRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(
		ruleset.And(
			isInPhase(t, Prevote),
			seenQuorumOfAnyPrevotes(t),
		),
	)
	rule.SetAction(func(Message) {
		go func() {
			select {
			case <-t.stopSignal:
			case <-t.nextRoundSignal:
				return
			case <-time.After(t.phaseTimeout[Prevote] + t.phaseTimeoutDelta*time.Duration(t.round)):
				t.stateMutex.Lock()
				defer t.stateMutex.Unlock()
				t.onTimeoutPrevote(t.height, t.round)
			}
		}()
	}).OnlyOnce()
	return &rule
}

func observePolkaOnProposalRule(t *Tendermint) *ruleset.Rule[Message] {
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
			msg := Message{
				Phase:     Precommit,
				Height:    t.height,
				Round:     t.round,
				BlockId:   proposal.Block.Id(),
				Signature: t.selfId,
			}
			go t.gossip.Broadcast(msg)
			t.currentPhase = Precommit
		}
		t.latestPolkaValue = proposal.Block
		t.latestPolkaRound = proposal.Round
	}).OnlyOnce()
	return &rule
}

func observeNilPrevoteQuorumRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(
		ruleset.And(
			isInPhase(t, Prevote),
			quorumOfNilPrevotes(t),
		),
	)
	rule.SetAction(func(Message) {
		msg := Message{
			Phase:     Precommit,
			Height:    t.height,
			Round:     t.round,
			BlockId:   types.Hash{}, // nil vote
			Signature: t.selfId,
		}
		go t.gossip.Broadcast(msg)
		t.currentPhase = Precommit
	})
	return &rule
}

func timeoutPrecommitRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(
		seenQuorumOfAnyPrecommits(t),
	)
	rule.SetAction(func(Message) {
		go func() {
			select {
			case <-t.stopSignal:
			case <-t.nextRoundSignal:
				return
			case <-time.After(t.phaseTimeout[Precommit] + t.phaseTimeoutDelta*time.Duration(t.round)):
				t.stateMutex.Lock()
				defer t.stateMutex.Unlock()
				t.onTimeoutPrecommit(t.height, t.round)
			}
		}()
	}).OnlyOnce()
	return &rule
}

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
		t.notifyListeners(types.Bundle{
			Number:       uint32(t.height),
			Transactions: proposal.Block.Transactions,
		})
		t.height++
		t.lockedValue = nil
		t.lockedRound = -1
		t.latestPolkaValue = nil
		t.latestPolkaRound = -1
		if t.heightLimit > 0 && t.height >= t.heightLimit {
			t.stop()
			return
		}
		delete(t.messageLog, t.height-1)
		t.startRound(0)
	})
	return &rule
}

func catchUpRule(t *Tendermint) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	var round int
	rule.SetCondition(atLeastOneHonestMessageFromLaterRound(t, &round))
	rule.SetAction(func(Message) {
		t.startRound(round)
	})
	return &rule
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
