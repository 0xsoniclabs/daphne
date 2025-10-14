package streamlet

import (
	"bytes"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

// The Streamlet consensus algorithm is a synchronous consensus protocol
// that tolerates up to f < n/3 Byzantine faults in a committee of n creators.
// It operates in synchronous rounds called epochs, each led by a designated leader.
// The current epoch is determined solely based on a common clock, which all nodes are
// assumed to share (equivalently, the nodes' clocks are assumed to be synchronized).
// The leader for each epoch is chosen solely based on the epoch number, in any
// deterministic fashion. This implementation opts for a round-robin approach.
// In each epoch, the leader proposes a block containing transactions to be added
// to the ledger. Other creators vote on the proposed block iff it extends one of
// the longest notarized chains they are aware of.
// A block is considered notarized if it receives a quorum of votes.
// When three notarized blocks with consecutive epoch numbers are chained, the whole
// chain, except for the latest block, gets finalized. Listeners get notified about the
// finalized blocks in the order they are finalized.
// All honest nodes are guaranteed to finalize the same blocks in the same order.
// This implementation strays from the canonical one in that it takes stake into account,
// while Streamlet traditionally assumes flat stake.

const (
	// DefaultEpochDuration is the default duration of each epoch
	// if one is not specified in the configuration.
	DefaultEpochDuration = 1 * time.Second
)

// Factory defines the configuration for the Streamlet consensus algorithm.
type Factory struct {
	// StartTime is the time when the first epoch starts.
	// It needs to be in the future, at least an EpochDuration from now.
	// The reason is to account for randomness in job start times.
	StartTime time.Time
	// EpochDuration is the duration of each epoch.
	EpochDuration time.Duration
	// Committee is the committee of creators participating in consensus.
	Committee consensus.Committee
	// SelfId is the CreatorId of the local node. Needs to be in the Committee.
	SelfId model.CreatorId
	// EmitProcedure is an arbitrary function run by the node's emitter.
	// It can be used to introduce faults for testing purposes.
	// If nil, the correct behavior is assumed.
	EmitProcedure func(*Streamlet, generic.EmissionPayloadSource[BlockMessage])
	// MessageHandleProcedure is an arbitrary function run when the node receives
	// a block message. It can be used to introduce faults for testing purposes.
	// If nil, the correct behavior is assumed.
	MessageHandleProcedure func(*Streamlet, BlockMessage)
}

// NewPassiveStreamlet creates a new passive Streamlet consensus instance.
// This instance does not create/emit bundles but listens for them from peers.
func (f Factory) NewPassive(p2pServer p2p.Server) consensus.Consensus {
	return newPassiveStreamlet(
		p2pServer,
		f.StartTime,
		f.EpochDuration,
		f.Committee,
		f.MessageHandleProcedure,
	)
}

// NewActive creates a new active Streamlet consensus instance.
func (f Factory) NewActive(p2pServer p2p.Server,
	source consensus.TransactionProvider) consensus.Consensus {
	return newActiveStreamlet(
		p2pServer,
		source,
		f.StartTime,
		f.EpochDuration,
		f.Committee,
		f.SelfId,
		f.MessageHandleProcedure,
		f.EmitProcedure,
	)
}

// Streamlet implements the Streamlet consensus algorithm.
type Streamlet struct {
	listenersMutex sync.Mutex
	listeners      []consensus.BundleListener

	startTime              time.Time
	epochDuration          time.Duration
	committee              consensus.Committee
	selfId                 model.CreatorId
	emitProcedure          func(*Streamlet, generic.EmissionPayloadSource[BlockMessage])
	messageHandleProcedure func(*Streamlet, BlockMessage)

	// hashToBlock maps block hashes to their corresponding BlockMessage.
	// It is a set of all blocks ever handled by the node.
	hashToBlock map[types.Hash]BlockMessage

	// longestNotarizedChains holds the hashes of the last blocks
	// of the longest notarized chains known to the node.
	// There may be multiple such chains of the same length (in case of forks).
	// The length of these chains is held in longestNotarizedChainsLength.
	longestNotarizedChains       []types.Hash
	longestNotarizedChainsLength int
	// votesForBlocks maps block hashes to their corresponding VoteCounter.
	votesForBlocks map[types.Hash]*consensus.VoteCounter
	// seenLeaderBlockThisEpoch is true if the node has already seen
	// a block from the leader of the current epoch. This is used to ensure
	// that the node votes for at most one block per epoch.
	seenLeaderBlockThisEpoch bool

	finalizedBlocks  map[types.Hash]struct{}
	nextBundleNumber uint32

	// orphanBlocks holds blocks that could not be handled immediately
	// due to missing parents.
	orphanBlocks []BlockMessage

	stateMutex sync.Mutex

	gossip  generic.Broadcaster[BlockMessage]
	emitter *generic.Emitter[BlockMessage]
}

// RegisterListener registers a listener to be notified of new bundles.
func (s *Streamlet) RegisterListener(listener consensus.BundleListener) {
	s.listenersMutex.Lock()
	defer s.listenersMutex.Unlock()
	s.listeners = append(s.listeners, listener)
}

// Stop stops the Streamlet consensus instance.
func (s *Streamlet) Stop() {
	if s.emitter != nil {
		s.emitter.Stop()
	}
}

func newPassiveStreamlet(
	p2pServer p2p.Server,
	startTime time.Time,
	epochDuration time.Duration,
	committee consensus.Committee,
	messageHandleProcedure func(*Streamlet, BlockMessage),
) *Streamlet {
	if epochDuration == 0 {
		epochDuration = DefaultEpochDuration
	}
	if time.Until(startTime) < epochDuration {
		startTime = time.Now().Add(epochDuration)
	}
	if messageHandleProcedure == nil {
		messageHandleProcedure = func(s *Streamlet, bm BlockMessage) {
			// Delay handling of blocks with unknown parents.
			s.orphanBlocks = append(s.orphanBlocks, bm)
			hasParent := func(bm BlockMessage) bool {
				_, exists := s.hashToBlock[bm.LastBlockHash]
				return exists
			}
			// Try exhausting the orphans, until no new orphans can be handled.
			for foundNew := true; foundNew; {
				foundNew = false
				for bm := range s.orphanBlocks {
					if hasParent(s.orphanBlocks[bm]) {
						s.handleBlock(s.orphanBlocks[bm])
						foundNew = true
						break
					}
				}
				// Remove handled orphans.
				s.orphanBlocks = slices.DeleteFunc(s.orphanBlocks, hasParent)
			}
		}
	}
	res := &Streamlet{
		startTime:              startTime,
		epochDuration:          epochDuration,
		committee:              committee,
		messageHandleProcedure: messageHandleProcedure,
		hashToBlock:            make(map[types.Hash]BlockMessage),
		votesForBlocks:         make(map[types.Hash]*consensus.VoteCounter),
		finalizedBlocks:        make(map[types.Hash]struct{}),
	}
	// Create genesis block.
	genesisBlock := BlockMessage{}
	res.addBlock(genesisBlock)
	// Notarize genesis block.
	for _, creator := range committee.Creators() {
		// Error ignored as it is guaranteed to not happen.
		res.votesForBlocks[genesisBlock.Hash()].Vote(creator)
	}
	res.longestNotarizedChains = []types.Hash{genesisBlock.Hash()}
	res.longestNotarizedChainsLength = 1
	res.finalizeBlock(genesisBlock.Hash())
	// Set up gossip.
	gossip := generic.NewGossip(
		p2pServer,
		func(bm BlockMessage) types.Hash { return bm.HashWithVoter() },
		p2p.MessageCode_StreamletConsensus_NewBlock,
	)
	gossip.RegisterReceiver(&onMessageAdapter{streamlet: res})
	res.gossip = gossip

	return res
}

func newActiveStreamlet(
	p2pServer p2p.Server,
	source consensus.TransactionProvider,
	startTime time.Time,
	epochDuration time.Duration,
	committee consensus.Committee,
	selfId model.CreatorId,
	messageHandleProcedure func(*Streamlet, BlockMessage),
	emitProcedure func(*Streamlet, generic.EmissionPayloadSource[BlockMessage]),
) *Streamlet {
	if emitProcedure == nil {
		emitProcedure = func(
			s *Streamlet,
			src generic.EmissionPayloadSource[BlockMessage],
		) {
			s.advanceEpoch(src)
		}
	}
	res := newPassiveStreamlet(
		p2pServer,
		startTime,
		epochDuration,
		committee,
		messageHandleProcedure,
	)
	res.selfId = selfId
	res.emitter = generic.StartCustomEmitter(epochDuration,
		emissionPayloadSourceAdapter{source: source, streamlet: res},
		res.gossip,
		func(_ time.Time,
			src generic.EmissionPayloadSource[BlockMessage],
			_ generic.Broadcaster[BlockMessage]) {
			emitProcedure(res, src)
		},
	)
	return res
}

// isValidator checks if the node is active by verifying if it is in the committee.
func (s *Streamlet) isValidator() bool {
	return slices.Contains(s.committee.Creators(), s.selfId)
}

// getEpoch calculates the current epoch based on the elapsed time since StartTime.
func (s *Streamlet) getEpoch() int {
	elapsed := time.Since(s.startTime)
	// If elapsed is negative, we are before the start time.
	if elapsed < 0 {
		return 0
	}
	return int(elapsed/s.epochDuration) + 1
}

// getLeader returns the CreatorId of the leader for the current epoch.
func (s *Streamlet) getLeader() model.CreatorId {
	creators := s.committee.Creators()
	epoch := s.getEpoch()
	if epoch == 0 {
		return model.CreatorId(0) // No leader in epoch 0.
	}
	return creators[(epoch-1)%len(creators)]
}

// advanceEpoch advances the epoch and, if the local node is the leader,
// creates a new block and broadcasts it to other validators.
func (s *Streamlet) advanceEpoch(source generic.EmissionPayloadSource[BlockMessage]) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.seenLeaderBlockThisEpoch = false
	if s.getLeader() == s.selfId {
		// Create a block and chain it to one of the longest notarized chains.
		blockMessage := source.GetEmissionPayload()
		s.gossip.Broadcast(blockMessage)
	}
}

// handleBlock gossips the received block message to peers, and attempts to vote
// on it if it is the first message from the leader of the current epoch,
// it extends one of the longest chains, and the node is active. If the block is
// notarized, it updates the information on the longest notarized chains and tries
// to finalize blocks.
func (s *Streamlet) handleBlock(bm BlockMessage) {
	// All nodes gossip all received blocks, even if passive.
	s.gossip.Broadcast(bm)
	// Store the block.
	s.addBlock(bm)
	// If message is the first one from the leader: vote on it (if active
	// and it extends the longest notarized chain).
	if s.isValidator() && bm.Voter == s.getLeader() &&
		extendsLongestNotarizedChain(s, bm) && !s.seenLeaderBlockThisEpoch {
		s.seenLeaderBlockThisEpoch = true
		s.gossip.Broadcast(BlockMessage{
			Epoch:         bm.Epoch,
			Transactions:  bm.Transactions,
			LastBlockHash: bm.LastBlockHash,
			Voter:         s.selfId,
		})
	}
	// If the block is notarized, update longest notarized chains.
	// Also, try finalizing blocks.
	if s.isNotarized(bm.Hash()) {
		s.chainBlock(bm)
		s.tryFinalizing(bm)
	}
}

// addBlock adds a block message to the local state, not chaining
// it to any existing chain. It also initializes the vote counter
// for the block if it does not already exist, and adds the vote
// from the sender of the message.
func (s *Streamlet) addBlock(bm BlockMessage) {
	voter := bm.Voter
	bm.Voter = model.CreatorId(0) // No voter info in hashToBlock.
	// Store the block.
	s.hashToBlock[bm.Hash()] = bm
	// Initialize vote counter if not present.
	if _, exists := s.votesForBlocks[bm.Hash()]; !exists {
		s.votesForBlocks[bm.Hash()] = consensus.NewVoteCounter(&s.committee)
	}
	// Add the vote from the sender. Error ignored as receiving a message from
	// a non-committee member should be ignored.
	_ = s.votesForBlocks[bm.Hash()].Vote(voter)
}

// chainBlock takes a notarized block message and updates the longest
// notarized chain data structures accordingly. bm is notarized.
func (s *Streamlet) chainBlock(bm BlockMessage) {
	// If the block extends one of the longest notarized chains,
	// extend that chain. It is now the sole longest notarized chain.
	if extendsLongestNotarizedChain(s, bm) {
		s.longestNotarizedChains = []types.Hash{bm.Hash()}
		s.longestNotarizedChainsLength++
		return
	}
	// If the block extends a notarized chain that is not the longest,
	// check if it is now one of the longest. If so, it is added to the list.
	var getChainLength func(types.Hash) int
	getChainLength = func(hash types.Hash) int {
		// Null hash means no block - termination of chain.
		if hash == (types.Hash{}) {
			return 0
		}
		return 1 + getChainLength(s.hashToBlock[hash].LastBlockHash)
	}
	chainLength := getChainLength(bm.LastBlockHash)
	if chainLength+1 == s.longestNotarizedChainsLength {
		s.longestNotarizedChains = append(s.longestNotarizedChains, bm.Hash())
	}
}

// tryFinalizing checks if a given notarized block message allows
// finalizing any blocks and finalizes them if so.
func (s *Streamlet) tryFinalizing(bm BlockMessage) {
	// The first ancestor is guaranteed to exist as bm cannot be genesis.
	firstAncestor := s.hashToBlock[bm.LastBlockHash]
	// The second ancestor might not exist if the first ancestor is genesis.
	secondAncestor, exists := s.hashToBlock[firstAncestor.LastBlockHash]
	if !exists {
		return
	}
	if bm.Epoch == firstAncestor.Epoch+1 &&
		firstAncestor.Epoch == secondAncestor.Epoch+1 {
		s.finalizeBlock(firstAncestor.Hash())
	}
}

// finalizeBlock finalizes the block with the given hash
// and recursively finalizes its ancestors if they are not already finalized.
func (s *Streamlet) finalizeBlock(hash types.Hash) {
	if _, alreadyFinalized := s.finalizedBlocks[hash]; alreadyFinalized {
		return
	}
	s.finalizedBlocks[hash] = struct{}{}
	prevBlock := s.hashToBlock[hash]
	if _, isFinalized := s.finalizedBlocks[prevBlock.LastBlockHash]; !isFinalized {
		s.finalizeBlock(prevBlock.Hash())
	}

	newBundle := types.Bundle{
		Number:       s.nextBundleNumber,
		Transactions: s.hashToBlock[hash].Transactions,
	}
	s.nextBundleNumber++
	s.notifyListeners(newBundle)
}

// isNotarized checks if a block with the given hash has reached quorum.
func (s *Streamlet) isNotarized(hash types.Hash) bool {
	return s.votesForBlocks[hash].IsQuorumReached()
}

// notifyListeners notifies all registered listeners of a new bundle.
func (s *Streamlet) notifyListeners(bundle types.Bundle) {
	s.listenersMutex.Lock()
	defer s.listenersMutex.Unlock()
	for _, listener := range s.listeners {
		listener.OnNewBundle(bundle)
	}
}

// selectChain provides deterministic selection of a chain from
// multiple chains of the same length.
func selectChain(chains []types.Hash) types.Hash {
	copy := slices.Clone(chains)
	slices.SortFunc(copy, func(a, b types.Hash) int {
		return bytes.Compare(a[:], b[:])
	})
	return copy[0]
}

// extendsLongestNotarizedChain checks if the block message extends
// any of the longest notarized chains.
func extendsLongestNotarizedChain(s *Streamlet, bm BlockMessage) bool {
	for _, hash := range s.longestNotarizedChains {
		if bm.LastBlockHash == hash {
			return true
		}
	}
	return false
}

type onMessageAdapter struct {
	streamlet *Streamlet
}

func (a *onMessageAdapter) OnMessage(bm BlockMessage) {
	a.streamlet.stateMutex.Lock()
	defer a.streamlet.stateMutex.Unlock()
	a.streamlet.messageHandleProcedure(a.streamlet, bm)
}

// BlockMessage represents a message containing transactions and the metadata.
// It includes the epoch, the transactions themselves,
// the hash of the last block, and the ID of the voter who sent the message
// (as every message is essentially a vote).
// Note: Voter is considered an unforgeable digital signature - faulty nodes
// may not misuse this field to impersonate other nodes.
type BlockMessage struct {
	Epoch         int
	Transactions  []types.Transaction
	LastBlockHash types.Hash
	Voter         model.CreatorId
}

// Hash computes a simple hash of the BlockMessage for identification purposes.
// It does not take Voter into account for the hash.
func (bm BlockMessage) Hash() types.Hash {
	data := fmt.Sprintf("%+v%+v%+v", bm.Epoch, bm.Transactions, bm.LastBlockHash)
	return types.Sha256([]byte(data))
}

// HashWithVoter computes a hash of the BlockMessage including the Voter field.
// This is useful for distinguishing messages from different voters, for gossiping.
func (bm BlockMessage) HashWithVoter() types.Hash {
	data := fmt.Sprintf("%+v", bm)
	return types.Sha256([]byte(data))
}

type emissionPayloadSourceAdapter struct {
	source    consensus.TransactionProvider
	streamlet *Streamlet
}

func (a emissionPayloadSourceAdapter) GetEmissionPayload() BlockMessage {
	transactions := a.source.GetCandidateTransactions()
	blockMessage := BlockMessage{
		Epoch:         a.streamlet.getEpoch(),
		Transactions:  transactions,
		LastBlockHash: selectChain(a.streamlet.longestNotarizedChains),
		Voter:         a.streamlet.selfId,
	}
	return blockMessage
}
