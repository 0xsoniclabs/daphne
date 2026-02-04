package hotstuff

import (
	"fmt"
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils/ruleset"
)

// Hotstuff is an implementation of the HotStuff 2 (not Hotstuff!) consensus
// protocol, as described in this paper: https://eprint.iacr.org/2023/397.pdf.
// It consists of multiple consecutive views, each led by a designated leader.
// These represent attempts to commit new blocks. The protocol can be summarized
// as follows:
// - Each view starts with its leader receiving 2f + 1 NewView messages from
//   validators, containing their highest known Quorum Certificates (QCs).
// - The leader selects the highest QC among these and proposes a new block
//   extending the block referenced by this QC.
// - Validators receiving a valid proposal vote for it by sending a Vote message
//   to the leader.
// - Upon collecting 2f + 1 votes for the proposed block, the leader forms a
//   Prepare QC and broadcasts it via a Prepare message.
// - Validators receiving a valid Prepare message update their locked QC and
//   advance to the next view, starting the process anew.
// The protocol includes certain optimizations to improve theoretical
// worst-case performance, but such optimizations are omitted here, similarly to
// real solutions - simpler techniques are used to ensure liveness (the so called
// "pacemaker" mechanism is ommitted in favor of a simple tau-based timeout).

const (
	defaultDelta = 500 * time.Millisecond
	defaultTau   = 7 * defaultDelta
)

// Factory is a factory for creating Hotstuff consensus instances.
type Factory struct {
	// Tau is the amount of time given to a leader to finish the view,
	// before the next view is started, failing to commit a block.
	// It should be set to at least 7 * Delta.
	Tau time.Duration
	// Delta is the estimated maximum network delay.
	// This algorithm assumes partial synchrony.
	Delta time.Duration
	// ViewLimit is an optional limit on the number of views to execute.
	// A value of 0 indicates no limit.
	ViewLimit uint64
	// BroadcastFactory is a factory for creating broadcast channels.
	// Used to inject custom broadcast implementations, primarily for testing.
	BroadcastFactory broadcast.Factory[types.Hash, Message]
}

func (f Factory) String() string {
	return fmt.Sprintf("hotstuff-tau=%.0fms-delta=%.0fms", f.Tau.Seconds()*1000, f.Delta.Seconds()*1000)
}

// normalizeTimings ensures that the timing parameters Tau and Delta
// are set to sensible defaults if not provided, and that Tau is at least
// 7 times Delta.
func (f *Factory) normalizeTimings() {
	// Set default Delta if not specified
	if f.Delta == 0 {
		f.Delta = defaultDelta
	}

	// Set default Tau if not specified (7 * Delta)
	if f.Tau == 0 {
		f.Tau = 7 * f.Delta
	}

	// Ensure Tau is at least 7 * Delta
	minTau := 7 * f.Delta
	if f.Tau < minTau {
		f.Tau = minTau
	}
}

// NewActive creates a new active Hotstuff consensus instance.
func (f Factory) NewActive(
	p2pServer p2p.Server,
	committee consensus.Committee,
	selfId consensus.ValidatorId,
	source consensus.TransactionProvider,
) consensus.Consensus {
	f.normalizeTimings()
	return newHotstuff(p2pServer, committee, selfId, f.Tau, f.Delta, f.ViewLimit, true, source, f.BroadcastFactory)
}

// NewPassive creates a new passive Hotstuff consensus instance.
func (f Factory) NewPassive(
	p2pServer p2p.Server,
	committee consensus.Committee,
) consensus.Consensus {
	f.normalizeTimings()
	return newHotstuff(p2pServer, committee, 0, f.Tau, f.Delta, f.ViewLimit, false, nil, f.BroadcastFactory)
}

func newHotstuff(
	p2pServer p2p.Server,
	committee consensus.Committee,
	selfId consensus.ValidatorId,
	tau time.Duration,
	delta time.Duration,
	viewLimit uint64,
	isActive bool,
	source consensus.TransactionProvider,
	broadcastFactory broadcast.Factory[types.Hash, Message],
) *Hotstuff {
	h := &Hotstuff{
		committee:     committee,
		selfId:        selfId,
		tau:           tau,
		delta:         delta,
		viewLimit:     viewLimit,
		isActive:      isActive,
		newViewBuffer: make(map[consensus.ValidatorId]certificate),
		blockStorage:  make(map[types.Hash]Block),
		source:        source,
	}
	h.ruleset = getHotstuffRuleset(h)
	h.receiver = broadcast.WrapReceiver(func(msg Message) {
		h.stateMutex.Lock()
		defer h.stateMutex.Unlock()
		h.handleMessage(msg)
	})
	if broadcastFactory == nil {
		broadcastFactory = broadcast.DefaultFactory[types.Hash, Message]()
	}
	h.gossip = broadcastFactory(p2pServer, func(msg Message) types.Hash {
		return msg.Hash()
	})
	h.gossip.Register(h.receiver)

	genesisBlock := genesisBlock(&h.committee)
	h.blockStorage[genesisBlock.Hash()] = genesisBlock
	genesisQC := certificate{
		view:       0,
		blockHash:  genesisBlock.Hash(),
		signatures: consensus.NewVoteCounter(&h.committee),
	}
	for _, id := range h.committee.Validators() {
		genesisQC.signatures.Vote(id)
	}

	h.lockedQC = genesisQC
	h.lastCommit = 0
	h.curView = 0

	h.stateMutex.Lock()
	h.advanceView(1)
	h.stateMutex.Unlock()

	return h
}

// Hotstuff implements the HotStuff consensus protocol.
type Hotstuff struct {
	listeners      []consensus.BundleListener
	listenersMutex sync.Mutex

	committee consensus.Committee
	selfId    consensus.ValidatorId
	isActive  bool

	// Timing parameters.
	delta time.Duration
	tau   time.Duration
	// Optional limit on the number of views to execute.
	viewLimit uint64

	// Current view number.
	curView uint64

	// Locked quorum certificate.
	lockedQC certificate

	// Last committed block's view number.
	lastCommit uint64

	// Buffers for incoming votes and new views.
	voteBuffer           map[types.Hash]*consensus.VoteCounter
	newViewBuffer        map[consensus.ValidatorId]certificate
	newViewQuorumCounter consensus.VoteCounter
	// Memory of blocks by their hash.
	blockStorage map[types.Hash]Block

	// Timer for a view timeout - tied to tau.
	viewTimer *time.Timer

	gossip   broadcast.Channel[Message]
	receiver broadcast.Receiver[Message]
	source   consensus.TransactionProvider

	stateMutex sync.Mutex
	stopped    bool

	// ruleset contains the set of rules defining the Hotstuff protocol.
	ruleset *ruleset.Ruleset[Message]
}

func (h *Hotstuff) RegisterListener(listener consensus.BundleListener) {
	h.listenersMutex.Lock()
	defer h.listenersMutex.Unlock()
	h.listeners = append(h.listeners, listener)
}

func (h *Hotstuff) Stop() {
	h.gossip.Unregister(h.receiver)
	h.stateMutex.Lock()
	defer h.stateMutex.Unlock()
	if h.viewTimer != nil {
		h.viewTimer.Stop()
	}
	h.stopped = true
}

// chooseLeader determines the leader for a given view, based on the committee.
// It uses a simple round-robin selection method.
func chooseLeader(view uint64, committee consensus.Committee) consensus.ValidatorId {
	ids := committee.Validators()
	return ids[view%uint64(len(ids))]
}

// handleMessage is called to process an incoming message.
// Assumes the state mutex is held.
func (h *Hotstuff) handleMessage(msg Message) {
	if !h.stopped {
		h.ruleset.Apply(msg)
	}
}

// advanceView advances the protocol to the specified view.
// Assumes the state mutex is held.
func (h *Hotstuff) advanceView(view uint64) {
	if h.viewTimer != nil {
		h.viewTimer.Stop()
	}
	// Check view limit
	if h.viewLimit > 0 && view > h.viewLimit {
		go h.Stop()
		return
	}
	h.curView = view
	h.voteBuffer = make(map[types.Hash]*consensus.VoteCounter)
	h.newViewBuffer = make(map[consensus.ValidatorId]certificate)
	h.newViewQuorumCounter = *consensus.NewVoteCounter(&h.committee)
	viewWhenStarted := h.curView
	h.viewTimer = time.AfterFunc(h.tau, func() {
		h.stateMutex.Lock()
		defer h.stateMutex.Unlock()
		if h.curView == viewWhenStarted {
			h.advanceView(h.curView + 1)
		}
	})
	msg := Message{
		Signature: h.selfId,
		Type:      NewView,
		View:      h.curView,
		Contents:  MessageNewViewContents{HighQC: h.lockedQC},
	}
	// TODO: Send to leader of h.curView only.
	if h.isActive {
		go h.gossip.Broadcast(msg)
	}
}

// proposeBlock creates and gossips a new block proposal extending the block
// referenced by the provided parentQC.
// Assumes the state mutex is held.
func (h *Hotstuff) proposeBlock(parentQC certificate) {
	transactions := h.source.GetCandidateLineup().All()
	block := Block{
		PrevHash: parentQC.blockHash,
		View:     h.curView,
		Justify:  parentQC,
		Payload:  transactions,
	}
	grandparentQC := certificate{signatures: consensus.NewVoteCounter(&h.committee)}
	if parent, ok := h.blockStorage[parentQC.blockHash]; ok {
		grandparentQC = parent.Justify
	}
	msg := Message{
		Signature: h.selfId,
		Type:      Propose,
		View:      h.curView,
		Contents: MessageProposeContents{
			Block:     block,
			High_QC:   parentQC,
			Commit_QC: grandparentQC,
		},
	}
	go h.gossip.Broadcast(msg)
}

// commitBlock commits the block with the given hash.
// Assumes the state mutex is held.
func (h *Hotstuff) commitBlock(blockHash types.Hash) {
	block := h.blockStorage[blockHash]

	h.lastCommit = block.View
	bundle := types.Bundle{
		Number:       uint32(block.View),
		Transactions: block.Payload,
	}

	h.listenersMutex.Lock()
	listeners := h.listeners
	h.listenersMutex.Unlock()

	for _, listener := range listeners {
		listener.OnNewBundle(bundle)
	}
}

// --- HOTSTUFF RULE CONDITIONS ---

// isLeader returns a condition function that checks if the current node
// is the leader for the current view.
func isLeader(h *Hotstuff) func(Message) bool {
	return func(msg Message) bool {
		return chooseLeader(h.curView, h.committee) == h.selfId && h.isActive
	}
}

// isLeaderInViewOfMessage returns a condition function that checks if the
// node is the leader for the view specified in the message.
func isLeaderInViewOfMessage(h *Hotstuff) func(Message) bool {
	return func(msg Message) bool {
		return chooseLeader(msg.View, h.committee) == h.selfId && h.isActive
	}
}

// messageIsOfType returns a condition function that checks if the message
// is of the specified type.
func messageIsOfType(msgType messageType) func(Message) bool {
	return func(msg Message) bool {
		return msg.Type == msgType
	}
}

// messageInCurrentView returns a condition function that checks if the
// message is for the current view.
func messageInCurrentView(h *Hotstuff) func(Message) bool {
	return func(msg Message) bool {
		return msg.View == h.curView
	}
}

// messageByLeader returns a condition function that checks if the message
// was sent by the leader for the current view.
func messageByLeader(h *Hotstuff) func(Message) bool {
	return func(msg Message) bool {
		return msg.Signature == chooseLeader(h.curView, h.committee)
	}
}

// --- HOTSTUFF RULES ---
// The rules are defined to handle the different message types.
func handleNewViewRule(h *Hotstuff) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(ruleset.And(
		messageIsOfType(NewView),
		isLeaderInViewOfMessage(h),
	))
	rule.SetAction(func(msg Message) {
		contents := msg.Contents.(MessageNewViewContents)
		h.newViewBuffer[msg.Signature] = contents.HighQC
		h.newViewQuorumCounter.Vote(msg.Signature)

		if h.newViewQuorumCounter.IsQuorumReached() {
			// In this context, the best QC is the one with the most recent view,
			// out of those that have been received by the leader for this view.
			// Ties resolved arbitrarily.
			getBestQC := func() certificate {
				bestView := uint64(0)
				bestQC := certificate{signatures: consensus.NewVoteCounter(&h.committee)}
				for _, qc := range h.newViewBuffer {
					if qc.view >= bestView {
						bestView = qc.view
						bestQC = qc
					}
				}
				return bestQC
			}
			bestQC := getBestQC()
			if bestQC.view == h.curView-1 {
				h.proposeBlock(bestQC)
			} else {
				viewToWaitFor := h.curView
				go func() {
					time.Sleep(3 * h.delta)
					h.stateMutex.Lock()
					defer h.stateMutex.Unlock()
					if h.curView == viewToWaitFor {
						bestQC = getBestQC()
						h.proposeBlock(bestQC)
					}
				}()
			}
		}
	})

	return &rule
}

func handleProposeRule(h *Hotstuff) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(ruleset.And(
		messageIsOfType(Propose),
		messageInCurrentView(h),
		messageByLeader(h),
	))
	rule.SetAction(func(msg Message) {
		contents := msg.Contents.(MessageProposeContents)
		h.blockStorage[contents.Block.Hash()] = contents.Block
		if (contents.Commit_QC.view > h.lastCommit || h.lastCommit == 0) &&
			contents.Commit_QC.blockHash != (types.Hash{}) &&
			contents.Commit_QC.signatures.IsQuorumReached() {
			h.commitBlock(contents.Commit_QC.blockHash)
		}
		if contents.Block.PrevHash == h.lockedQC.blockHash ||
			contents.High_QC.view >= h.lockedQC.view {
			h.lockedQC = contents.High_QC
			newMsg := Message{
				Signature: h.selfId,
				Type:      Vote,
				View:      h.curView,
				Contents: MessageVoteContents{
					BlockHash: contents.Block.Hash(),
				},
			}
			go h.gossip.Broadcast(newMsg)
		}
	})

	return &rule
}

func handleVoteRule(h *Hotstuff) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(ruleset.And(
		messageIsOfType(Vote),
		messageInCurrentView(h),
		isLeader(h),
	))
	rule.SetAction(func(msg Message) {
		contents := msg.Contents.(MessageVoteContents)
		if _, exists := h.voteBuffer[contents.BlockHash]; !exists {
			h.voteBuffer[contents.BlockHash] = consensus.NewVoteCounter(&h.committee)
		}
		h.voteBuffer[contents.BlockHash].Vote(msg.Signature)
		if h.voteBuffer[contents.BlockHash].IsQuorumReached() {
			qc := certificate{
				view:       h.curView,
				blockHash:  contents.BlockHash,
				signatures: h.voteBuffer[contents.BlockHash].Clone(),
			}
			if h.isActive {
				newMsg := Message{
					Signature: h.selfId,
					Type:      Prepare,
					View:      h.curView,
					Contents: MessagePrepareContents{
						PrepareQC: qc,
					},
				}
				go h.gossip.Broadcast(newMsg)
			}
		}
	})

	return &rule
}

func handlePrepareRule(h *Hotstuff) *ruleset.Rule[Message] {
	rule := ruleset.Rule[Message]{}
	rule.SetCondition(ruleset.And(
		messageIsOfType(Prepare),
		messageInCurrentView(h),
		messageByLeader(h),
	))
	rule.SetAction(func(msg Message) {
		contents := msg.Contents.(MessagePrepareContents)
		h.lockedQC = contents.PrepareQC
		h.lockedQC.signatures = h.lockedQC.signatures.Clone()
		h.advanceView(h.curView + 1)
	})

	return &rule
}

func getHotstuffRuleset(h *Hotstuff) *ruleset.Ruleset[Message] {
	rs := ruleset.Ruleset[Message]{}

	rs.AddRule(handleNewViewRule(h), 0)
	rs.AddRule(handleProposeRule(h), 0)
	rs.AddRule(handleVoteRule(h), 0)
	rs.AddRule(handlePrepareRule(h), 0)

	return &rs
}

// --- HELPER TYPES ---

// certificate represents a quorum certificate in the HotStuff protocol.
// It contains the view number, the hash of the block it certifies,
// and the signatures (votes) from validators.
type certificate struct {
	view       uint64
	blockHash  types.Hash
	signatures *consensus.VoteCounter
}

// Block represents a block in the HotStuff protocol.
type Block struct {
	// Hash of the previous block.
	PrevHash types.Hash
	// View number of this block.
	View uint64
	// Justify represents the QC of the parent block.
	Justify certificate
	// The contents of the block.
	Payload []types.Transaction
}

// Hash computes the hash of the block deterministically.
func (b Block) Hash() types.Hash {
	justifyForHash := struct {
		view            uint64
		blockHash       types.Hash
		certificateHash types.Hash
	}{
		view:            b.Justify.view,
		blockHash:       b.Justify.blockHash,
		certificateHash: b.Justify.signatures.Hash(),
	}
	if len(b.Payload) == 0 {
		b.Payload = nil
	}
	data := fmt.Sprintf("%+v%+v%+v%+v", b.PrevHash, b.View, justifyForHash, b.Payload)
	return types.Sha256([]byte(data))
}

// genesisBlock creates the genesis block for the HotStuff protocol.
func genesisBlock(committee *consensus.Committee) Block {
	return Block{
		View:     0,
		PrevHash: types.Hash{},
		Justify:  certificate{signatures: getFullVoteCounter(committee)},
	}
}

// getFullVoteCounter creates a VoteCounter with votes from all validators.
func getFullVoteCounter(committee *consensus.Committee) *consensus.VoteCounter {
	vc := consensus.NewVoteCounter(committee)
	for _, v := range committee.Validators() {
		vc.Vote(v)
	}
	return vc
}

// messageType represents the type of a HotStuff message.
type messageType int

const (
	Propose messageType = iota
	Vote
	Prepare
	NewView
)

// Message represents a HotStuff protocol message.
// Its contents vary based on the message type.
type Message struct {
	Signature consensus.ValidatorId
	Type      messageType
	View      uint64
	Contents  any
}

// Hash computes a hash of the message for identification in gossip.
func (m Message) Hash() types.Hash {
	data := fmt.Sprintf("%+v", m)
	return types.Sha256([]byte(data))
}

// MessageProposeContents represents the contents of a Propose message.
type MessageProposeContents struct {
	// The block being proposed.
	Block Block
	// The highest QC known to the proposer.
	High_QC certificate
	// The QC for the grandparent block, that can be committed.
	Commit_QC certificate
}

// MessageVoteContents represents the contents of a Vote message.
type MessageVoteContents struct {
	// The block being voted on.
	BlockHash types.Hash
}

// MessagePrepareContents represents the contents of a Prepare message.
type MessagePrepareContents struct {
	// The QC formed from the collected votes.
	PrepareQC certificate
}

// MessageNewViewContents represents the contents of a NewView message.
type MessageNewViewContents struct {
	// The sender's high QC.
	HighQC certificate
}
