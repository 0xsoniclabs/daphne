// Copyright 2026 Sonic Labs
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

package streamlet

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

// The Streamlet consensus algorithm is a synchronous consensus protocol
// that tolerates up to f < n/3 Byzantine faults in a committee of n creators.
// It operates in synchronous rounds called epochs, each led by a designated leader.
// The current epoch is determined solely based on a common clock, which all nodes are
// assumed to share (or equivalently, the nodes' clocks are assumed to be synchronized).
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
//
// The algorithm is derived from Chapter 7 of
// https://elaineshi.com/docs/blockchain-book.pdf
// in which additional material and proofs can be found.

const (
	// DefaultEpochDuration is the default duration of each epoch
	// if one is not specified in the configuration.
	DefaultEpochDuration = 1 * time.Second
)

// Factory defines the configuration for the Streamlet consensus algorithm.
type Factory struct {
	// EpochDuration is the duration of each epoch.
	EpochDuration time.Duration
	// EmitProcedure is an arbitrary function run by the node's emitter.
	// It can be used to introduce faults for testing purposes.
	// If nil, the correct behavior is assumed.
	EmitProcedure func(*Streamlet, consensus.TransactionProvider)
}

// NewPassive creates a new passive Streamlet consensus instance.
// This instance does not create/emit bundles but listens for them from peers.
func (f Factory) NewPassive(
	p2pServer p2p.Server,
	committee consensus.Committee,
) consensus.Consensus {
	epochDuration := f.EpochDuration
	if epochDuration == 0 {
		epochDuration = DefaultEpochDuration
	}
	return newPassiveStreamlet(
		p2pServer,
		epochDuration,
		committee,
	)
}

// NewActive creates a new active Streamlet consensus instance.
func (f Factory) NewActive(
	p2pServer p2p.Server,
	committee consensus.Committee,
	selfId consensus.ValidatorId,
	source consensus.TransactionProvider,
) consensus.Consensus {
	epochDuration := f.EpochDuration
	if epochDuration == 0 {
		epochDuration = DefaultEpochDuration
	}
	return newActiveStreamlet(
		p2pServer,
		source,
		epochDuration,
		committee,
		selfId,
		f.EmitProcedure,
	)
}

func (f Factory) String() string {
	epochDuration := f.EpochDuration
	if epochDuration == 0 {
		epochDuration = DefaultEpochDuration
	}
	return fmt.Sprintf("streamlet-%.0fms", epochDuration.Seconds()*1000)
}

// Streamlet implements the Streamlet consensus algorithm.
type Streamlet struct {
	listeners *consensus.BundleListenerManager

	epochDuration time.Duration
	committee     consensus.Committee
	selfId        consensus.ValidatorId

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

	finalizedBlocks  sets.Set[types.Hash]
	nextBundleNumber uint32

	// orphanBlocks holds blocks that could not be handled immediately
	// due to missing parents.
	orphanBlocks []BlockMessage

	stateMutex sync.Mutex

	channel  broadcast.Channel[BlockMessage]
	receiver broadcast.Receiver[BlockMessage]
	emitter  atomic.Pointer[concurrent.Job]
}

// RegisterListener registers a listener to be notified of new bundles.
func (s *Streamlet) RegisterListener(listener consensus.BundleListener) {
	s.listeners.RegisterListener(listener)
}

// Stop stops the Streamlet consensus instance.
func (s *Streamlet) Stop() {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.channel.Unregister(s.receiver)
	if emitter := s.emitter.Load(); emitter != nil {
		emitter.Stop()
	}

	if s.listeners != nil {
		s.listeners.Stop()
		s.listeners = nil
	}
}

func newPassiveStreamlet(
	p2pServer p2p.Server,
	epochDuration time.Duration,
	committee consensus.Committee,
) *Streamlet {
	res := &Streamlet{
		listeners:        consensus.NewBundleListenerManager(),
		epochDuration:    epochDuration,
		committee:        committee,
		hashToBlock:      make(map[types.Hash]BlockMessage),
		votesForBlocks:   make(map[types.Hash]*consensus.VoteCounter),
		nextBundleNumber: 1,
	}
	// Create genesis block.
	genesisBlock := BlockMessage{}
	res.addBlock(genesisBlock)
	// Notarize genesis block.
	for _, creator := range committee.Validators() {
		res.votesForBlocks[genesisBlock.Hash()].Vote(creator)
	}
	res.longestNotarizedChains = []types.Hash{genesisBlock.Hash()}
	res.longestNotarizedChainsLength = 1
	res.finalizeBlock(genesisBlock.Hash())
	// Set up gossip.
	res.receiver = broadcast.WrapReceiver(func(bm BlockMessage) {
		res.stateMutex.Lock()
		defer res.stateMutex.Unlock()
		handleMessage(res, bm)
	})
	channel := broadcast.NewGossip(
		p2pServer,
		func(bm BlockMessage) types.Hash { return bm.HashWithVoter() },
	)
	channel.Register(res.receiver)
	res.channel = channel

	return res
}

func newActiveStreamlet(
	p2pServer p2p.Server,
	source consensus.TransactionProvider,
	epochDuration time.Duration,
	committee consensus.Committee,
	selfId consensus.ValidatorId,
	emitProcedure func(*Streamlet, consensus.TransactionProvider),
) *Streamlet {
	if emitProcedure == nil {
		emitProcedure = defaultEmitProcedure
	}
	res := newPassiveStreamlet(
		p2pServer,
		epochDuration,
		committee,
	)
	res.stateMutex.Lock()
	res.selfId = selfId
	res.stateMutex.Unlock()
	res.emitter.Store(concurrent.StartPeriodicJob(
		epochDuration,
		func(time.Time) {
			res.stateMutex.Lock()
			defer res.stateMutex.Unlock()
			emitProcedure(res, source)
		},
	))
	return res
}

// isValidator checks if the node is active by verifying if it is in the committee,
// and whether it is active.
func (s *Streamlet) isValidator() bool {
	return slices.Contains(s.committee.Validators(), s.selfId) && s.emitter.Load() != nil
}

// getEpoch calculates the current epoch based on the current system time.
func (s *Streamlet) getEpoch() int {
	now := time.Now()
	return int(now.UnixNano()/s.epochDuration.Nanoseconds()) + 1
}

// advanceEpoch advances the epoch and, if the local node is the leader,
// creates a new block and broadcasts it to other validators.
// The caller is assumed to hold stateMutex.
func (s *Streamlet) advanceEpoch(source consensus.TransactionProvider) {
	s.seenLeaderBlockThisEpoch = false
	if chooseLeader(s.getEpoch(), s.committee) == s.selfId {
		// Create a block and chain it to one of the longest notarized chains.
		blockMessage := s.createProposalMessage(source)
		s.channel.Broadcast(blockMessage)
	}
}

// createProposalMessage creates a new block message making a new block proposal
// for the current epoch.
func (s *Streamlet) createProposalMessage(
	source consensus.TransactionProvider,
) BlockMessage {
	transactions := source.GetCandidateLineup().All()
	blockMessage := BlockMessage{
		Epoch:         s.getEpoch(),
		Transactions:  transactions,
		LastBlockHash: selectChain(s.longestNotarizedChains),
		Voter:         s.selfId,
	}
	return blockMessage
}

// handleBlock gossips the received block message to peers, and attempts to vote
// on it if it is the first message from the leader of the current epoch,
// it extends one of the longest chains, and the node is active. If the block is
// notarized, it updates the information on the longest notarized chains and tries
// to finalize blocks.
// The caller is assumed to hold stateMutex.
func (s *Streamlet) handleBlock(bm BlockMessage) {
	// Store the block.
	s.addBlock(bm)
	// If message is the first one from the leader: vote on it (if active
	// and it extends the longest notarized chain).
	if s.isValidator() && bm.Voter == chooseLeader(s.getEpoch(), s.committee) &&
		extendsLongestNotarizedChain(s, bm) && !s.seenLeaderBlockThisEpoch {
		s.seenLeaderBlockThisEpoch = true
		s.channel.Broadcast(BlockMessage{
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
// The caller is assumed to hold stateMutex.
func (s *Streamlet) addBlock(bm BlockMessage) {
	voter := bm.Voter
	bm.Voter = consensus.ValidatorId(0) // No voter info in hashToBlock.
	// Store the block.
	s.hashToBlock[bm.Hash()] = bm
	// Initialize vote counter if not present.
	if _, exists := s.votesForBlocks[bm.Hash()]; !exists {
		s.votesForBlocks[bm.Hash()] = consensus.NewVoteCounter(&s.committee)
	}
	// Add the vote from the sender. Error ignored as receiving a message from
	// a non-committee member should be ignored.
	s.votesForBlocks[bm.Hash()].Vote(voter)
}

// chainBlock takes a notarized block message and updates the longest
// notarized chain data structures accordingly. bm is notarized.
// The caller is assumed to hold stateMutex.
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
// The caller is assumed to hold stateMutex.
func (s *Streamlet) tryFinalizing(bm BlockMessage) {
	// The first ancestor is guaranteed to exist as bm cannot be genesis.
	firstAncestor := s.hashToBlock[bm.LastBlockHash]
	// The second ancestor might not exist if the first ancestor is genesis.
	secondAncestor, exists := s.hashToBlock[firstAncestor.LastBlockHash]
	if !exists {
		return
	}
	// Check if the three blocks are in consecutive epochs or include genesis.
	if bm.Epoch == firstAncestor.Epoch+1 &&
		(firstAncestor.Epoch == secondAncestor.Epoch+1 || secondAncestor.Epoch == 0) {
		s.finalizeBlock(firstAncestor.Hash())
	}
}

// finalizeBlock finalizes the block with the given hash
// and recursively finalizes its ancestors if they are not already finalized.
// The caller is assumed to hold stateMutex.
func (s *Streamlet) finalizeBlock(hash types.Hash) {
	if s.finalizedBlocks.Contains(hash) {
		return
	}
	s.finalizedBlocks.Add(hash)
	if hash == (BlockMessage{}).Hash() {
		return // Genesis block's parent is nil.
	}
	block := s.hashToBlock[hash]
	s.finalizeBlock(block.LastBlockHash)

	newBundle := types.Bundle{
		Number:       s.nextBundleNumber,
		Transactions: s.hashToBlock[hash].Transactions,
	}
	s.nextBundleNumber++
	s.notifyListeners(newBundle)
}

// isNotarized checks if a block with the given hash has reached quorum.
// The caller is assumed to hold stateMutex.
func (s *Streamlet) isNotarized(hash types.Hash) bool {
	return s.votesForBlocks[hash].IsQuorumReached()
}

// notifyListeners notifies all registered listeners of a new bundle.
func (s *Streamlet) notifyListeners(bundle types.Bundle) {
	s.listeners.NotifyListeners(bundle)
}

func chooseLeader(epoch int, committee consensus.Committee) consensus.ValidatorId {
	creators := committee.Validators()
	return creators[(epoch-1)%len(creators)]
}

// The caller is assumed to hold stateMutex.
func handleMessage(s *Streamlet, bm BlockMessage) {
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

// The caller is assumed to hold stateMutex.
func defaultEmitProcedure(s *Streamlet, source consensus.TransactionProvider) {
	s.advanceEpoch(source)
}

// selectChain provides deterministic selection of a chain from
// multiple chains of the same length.
// The caller is assumed to hold stateMutex.
func selectChain(chains []types.Hash) types.Hash {
	copy := slices.Clone(chains)
	slices.SortFunc(copy, func(a, b types.Hash) int {
		return bytes.Compare(a[:], b[:])
	})
	return copy[0]
}

// extendsLongestNotarizedChain checks if the block message extends
// any of the longest notarized chains.
// The caller is assumed to hold stateMutex.
func extendsLongestNotarizedChain(s *Streamlet, bm BlockMessage) bool {
	return slices.Contains(s.longestNotarizedChains, bm.LastBlockHash)
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
	Voter         consensus.ValidatorId
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

// MessageSize computes the size of the BlockMessage in bytes.
func (bm BlockMessage) MessageSize() uint32 {
	res := uint32(reflect.TypeFor[BlockMessage]().Size())
	for _, tx := range bm.Transactions {
		res += tx.MessageSize()
	}
	return res
}
