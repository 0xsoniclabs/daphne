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

const (
	// DefaultEpochDuration is the default duration of each epoch
	// if one is not specified in the configuration.
	DefaultEpochDuration = 1 * time.Second
)

// Factory defines the configuration for the Streamlet consensus algorithm.
type Factory struct {
	// EpochDuration is the duration of each epoch.
	EpochDuration time.Duration
	// Committee is the committee of creators participating in consensus.
	Committee consensus.Committee
	// SelfId is the CreatorId of the local node. Needs to be in the Committee.
	SelfId model.CreatorId
}

// NewPassiveStreamlet creates a new passive Streamlet consensus instance.
// This instance does not create/emit bundles but listens for them from peers.
func (f Factory) NewPassive(p2pServer p2p.Server) consensus.Consensus {
	if f.EpochDuration == 0 {
		f.EpochDuration = DefaultEpochDuration
	}
	return newPassiveStreamlet(p2pServer, &f)
}

// NewActive creates a new active Streamlet consensus instance.
func (f Factory) NewActive(p2pServer p2p.Server,
	source consensus.TransactionProvider) consensus.Consensus {
	return newActiveStreamlet(p2pServer, source, &f)
}

// Streamlet implements the Streamlet consensus algorithm.
type Streamlet struct {
	p2p            p2p.Server
	listenersMutex sync.Mutex
	listeners      []consensus.BundleListener
	config         *Factory

	epoch int

	hashToBlock map[types.Hash]BlockMessage

	longestNotarizedChains       []types.Hash
	longestNotarizedChainsLength int
	votesForBlocks               map[types.Hash]*consensus.VoteCounter

	finalizedBlocks  map[types.Hash]struct{}
	nextBundleNumber uint32

	stateMutex sync.Mutex

	gossip  generic.Gossip[BlockMessage]
	emitter *generic.Emitter[BlockMessage]
}

func (s *Streamlet) RegisterListener(listener consensus.BundleListener) {
	s.listenersMutex.Lock()
	defer s.listenersMutex.Unlock()
	s.listeners = append(s.listeners, listener)
}

func (s *Streamlet) Stop() {
	if s.emitter != nil {
		s.emitter.Stop()
	}
}

func newPassiveStreamlet(
	p2pServer p2p.Server,
	config *Factory,
) *Streamlet {
	res := &Streamlet{
		p2p:             p2pServer,
		config:          config,
		hashToBlock:     make(map[types.Hash]BlockMessage),
		votesForBlocks:  make(map[types.Hash]*consensus.VoteCounter),
		finalizedBlocks: make(map[types.Hash]struct{}),
	}
	// Create genesis block.
	genesisBlock := BlockMessage{}
	res.addBlock(genesisBlock)
	// Notarize genesis block.
	for _, creator := range config.Committee.Creators() {
		// Error ignored as it is guaranteed to not happen.
		_ = res.votesForBlocks[genesisBlock.Hash()].Vote(creator)
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
	config *Factory,
) *Streamlet {
	res := newPassiveStreamlet(p2pServer, config)
	res.emitter = generic.StartCustomEmitter(config.EpochDuration,
		emissionPayloadSourceAdapter{source: source, streamlet: res},
		res.gossip,
		func(_ time.Time,
			src generic.EmissionPayloadSource[BlockMessage],
			_ generic.Broadcaster[BlockMessage]) {
			res.advanceEpoch(src)
		},
	)
	return res
}

// isActive checks if the node is active by verifying if it is in the committee.
func (s *Streamlet) isActive() bool {
	return slices.Contains(s.config.Committee.Creators(), s.config.SelfId)
}

// getLeader returns the CreatorId of the leader for the current epoch.
func (s *Streamlet) getLeader() model.CreatorId {
	creators := s.config.Committee.Creators()
	return creators[s.epoch%len(creators)]
}

// advanceEpoch advances the epoch and, if the local node is the leader,
// creates a new block and broadcasts it to other validators.
func (s *Streamlet) advanceEpoch(source generic.EmissionPayloadSource[BlockMessage]) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.epoch++
	if s.getLeader() == s.config.SelfId {
		s.createBlock(source)
	}
}

// createBlock creates a new block from candidate transactions
// and gossips it to the network. It chains the new block to one of the
// longest notarized chains, chosen deterministically, yet arbitrarily.
func (s *Streamlet) createBlock(source generic.EmissionPayloadSource[BlockMessage]) {
	// Create a block and chain it to one of the longest notarized chains.
	blockMessage := source.GetEmissionPayload()
	// Update chain state.
	s.longestNotarizedChains = []types.Hash{blockMessage.Hash()}
	s.longestNotarizedChainsLength++

	s.addBlock(blockMessage)

	s.gossip.Broadcast(blockMessage)
}

// handleBlock gossips the received block message to peers,
// notifies local listeners, and processes it if the node is active.
func (s *Streamlet) handleBlock(bm BlockMessage) {
	// All nodes gossip all received blocks, even if inactive.
	// Processing and voting is done only if active.
	s.gossip.Broadcast(bm)
	if s.isActive() {
		s.processBlock(bm)
	}
}

// processBlock processes a received block message.
// It adds the block to the local state, checks if it belongs to the longest
// notarized chain, and votes for it if so while updating local state accordingly.
func (s *Streamlet) processBlock(bm BlockMessage) {
	// Ignore blocks from other epochs.
	if bm.Epoch != s.epoch {
		return
	}
	s.addBlock(bm)
	// Get length of the chain the new block belongs to.
	chainLength, err := s.chainLength(bm)
	// Ignore blocks with unknown parents.
	if err != nil {
		return
	}
	// If the chain is the longest, vote for the block and set it as longest.
	if chainLength > s.longestNotarizedChainsLength {
		s.longestNotarizedChains = []types.Hash{bm.Hash()}
		s.longestNotarizedChainsLength = chainLength
		// Vote by sending a block message with own ID as voter.
		voteBlock := bm
		voteBlock.Voter = s.config.SelfId
		s.gossip.Broadcast(voteBlock)
	} else if chainLength == s.longestNotarizedChainsLength {
		// If the chain is tied for longest, add it to the list of longest chains.
		s.longestNotarizedChains = append(s.longestNotarizedChains, bm.Hash())
	}

}

// addBlock adds a block message to the local state,
// initializes its vote counter if not present, and checks
// if it can be finalized.
func (s *Streamlet) addBlock(bm BlockMessage) {
	voter := bm.Voter
	bm.Voter = model.CreatorId(0) // No voter info in hashToBlock.
	// Store the block.
	s.hashToBlock[bm.Hash()] = bm
	// Initialize vote counter if not present.
	if _, exists := s.votesForBlocks[bm.Hash()]; !exists {
		s.votesForBlocks[bm.Hash()] = consensus.NewVoteCounter(&s.config.Committee)
	}
	// Add the vote from the sender. Error ignored as receiving a message from
	// a non-committee member should be ignored.
	_ = s.votesForBlocks[bm.Hash()].Vote(voter)

	// Check if blocks can be finalized.
	// If there are three consecutive blocks with consecutive epochs in a notarized chain,
	// the whole subchain can be finalized, except the latest block.
	chainLength, _ := s.chainLength(bm)
	if chainLength < 3 {
		return
	}
	latestThreeBlocks := []BlockMessage{bm, {}, {}}
	latestThreeBlocks[1] = s.hashToBlock[bm.LastBlockHash]
	latestThreeBlocks[2] = s.hashToBlock[latestThreeBlocks[1].LastBlockHash]
	if latestThreeBlocks[0].Epoch == latestThreeBlocks[1].Epoch+1 &&
		latestThreeBlocks[1].Epoch == latestThreeBlocks[2].Epoch+1 {
		chainIsNotarized := true
		curBlock := bm
		for {
			// Found an unnotarized block, stop.
			if !s.isNotarized(curBlock.Hash()) {
				chainIsNotarized = false
				break
			}
			// Reached a finalized block, subchain is notarized.
			// Genesis is always finalized, meaning this will always terminate.
			if _, isFinalized := s.finalizedBlocks[curBlock.Hash()]; isFinalized {
				break
			}
			curBlock = s.hashToBlock[curBlock.LastBlockHash]
		}
		if chainIsNotarized {
			s.finalizeBlock(bm.Hash())
		}
	}
}

// finalizeBlock finalizes the block with the given hash
// and recursively finalizes its ancestors if they are not already finalized.
func (s *Streamlet) finalizeBlock(hash types.Hash) {
	if _, alreadyFinalized := s.finalizedBlocks[hash]; alreadyFinalized {
		return
	}
	s.finalizedBlocks[hash] = struct{}{}
	prevBlock, exists := s.hashToBlock[hash]
	if exists {
		if _, isFinalized := s.finalizedBlocks[prevBlock.LastBlockHash]; !isFinalized {
			s.finalizeBlock(prevBlock.Hash())
		}
	}
	s.nextBundleNumber++
	newBundle := types.Bundle{
		Number:       s.nextBundleNumber,
		Transactions: s.hashToBlock[hash].Transactions,
	}
	s.notifyListeners(newBundle)
}

// isNotarized checks if a block with the given hash has reached quorum.
func (s *Streamlet) isNotarized(hash types.Hash) bool {
	return s.votesForBlocks[hash].IsQuorumReached()
}

// chainLength recursively computes the length of the chain
// ending with this block message. It stops when it reaches a genesis block.
func (s *Streamlet) chainLength(bm BlockMessage) (int, error) {
	if bm.LastBlockHash == (types.Hash{}) {
		return 1, nil
	}
	parent, exists := s.hashToBlock[bm.LastBlockHash]
	if !exists {
		return 0, fmt.Errorf("parent block not found")
	}
	len, err := s.chainLength(parent)
	if err != nil {
		return 0, err
	}
	return 1 + len, nil
}

// notifyListeners notifies all registered listeners of a new bundle.
func (s *Streamlet) notifyListeners(bundle types.Bundle) {
	s.listenersMutex.Lock()
	defer s.listenersMutex.Unlock()
	for _, listener := range s.listeners {
		listener.OnNewBundle(bundle)
	}
}

// A deterministic selection of a chain from multiple chains of the same length.
func selectChain(chains []types.Hash) types.Hash {
	copy := slices.Clone(chains)
	slices.SortFunc(copy, func(a, b types.Hash) int {
		return bytes.Compare(a[:], b[:])
	})
	return copy[0]
}

type onMessageAdapter struct {
	streamlet *Streamlet
}

func (a *onMessageAdapter) OnMessage(bm BlockMessage) {
	a.streamlet.handleBlock(bm)
}

// BlockMessage represents a message containing transactions and the metadata.
// It includes the epoch, the transactions themselves,
// the hash of the last block, and the ID of the voter who sent the message
// (as every message is essentially a vote).
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
		Epoch:         a.streamlet.epoch,
		Transactions:  transactions,
		LastBlockHash: selectChain(a.streamlet.longestNotarizedChains),
		Voter:         a.streamlet.config.SelfId,
	}
	return blockMessage
}
