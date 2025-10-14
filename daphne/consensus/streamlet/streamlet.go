package streamlet

import (
	"bytes"
	"fmt"
	"log/slog"
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

	hashToBundle map[types.Hash]BundleMessage

	longestNotarizedChains       []types.Hash
	longestNotarizedChainsLength int
	votesForBundles              map[types.Hash]*consensus.VoteCounter

	finalizedBundles map[types.Hash]struct{}

	stateMutex sync.Mutex

	gossip generic.Gossip[BundleMessage]
}

func (s *Streamlet) RegisterListener(listener consensus.BundleListener) {
	s.listenersMutex.Lock()
	defer s.listenersMutex.Unlock()
	s.listeners = append(s.listeners, listener)
}

func newPassiveStreamlet(
	p2pServer p2p.Server,
	config *Factory,
) *Streamlet {
	res := &Streamlet{
		p2p:              p2pServer,
		config:           config,
		hashToBundle:     make(map[types.Hash]BundleMessage),
		votesForBundles:  make(map[types.Hash]*consensus.VoteCounter),
		finalizedBundles: make(map[types.Hash]struct{}),
	}
	// Create genesis bundle.
	genesisBundle := BundleMessage{}
	res.addBundle(genesisBundle)
	// Notarize genesis bundle.
	for _, creator := range config.Committee.Creators() {
		// Error ignored as it is guaranteed to not happen.
		_ = res.votesForBundles[genesisBundle.Hash()].Vote(creator)
	}
	res.longestNotarizedChains = []types.Hash{genesisBundle.Hash()}
	res.longestNotarizedChainsLength = 1
	res.finalizeBundle(genesisBundle.Hash())
	// Set up gossip.
	gossip := generic.NewGossip(
		p2pServer,
		func(bm BundleMessage) types.Hash { return bm.HashWithVoter() },
		p2p.MessageCode_StreamletConsensus_NewBundle,
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
	go func() {
		for {
			time.Sleep(config.EpochDuration)
			res.advanceEpoch(source)
		}
	}()
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
// creates and emits a new bundle.
func (s *Streamlet) advanceEpoch(source consensus.TransactionProvider) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.epoch++
	if s.getLeader() == s.config.SelfId {
		s.emitBundle(source)
	}
}

// emitBundle creates a new bundle from candidate transactions
// and gossips it to the network. It chains the new bundle to one of the
// longest notarized chains, chosen deterministically, yet arbitrarily.
func (s *Streamlet) emitBundle(source consensus.TransactionProvider) {
	// Create a bundle and chain it to one of the longest notarized chains.
	transactions := source.GetCandidateTransactions()
	bundle := types.Bundle{
		Transactions: transactions,
	}
	bundleMessage := BundleMessage{
		Epoch:          s.epoch,
		Bundle:         bundle,
		LastBundleHash: selectChain(s.longestNotarizedChains),
		Voter:          s.config.SelfId,
	}
	// Update chain state.
	s.longestNotarizedChains = []types.Hash{bundleMessage.Hash()}
	s.longestNotarizedChainsLength++

	s.addBundle(bundleMessage)

	s.gossip.Broadcast(bundleMessage)
}

// handleBundle gossips the received bundle message to peers,
// notifies local listeners, and processes it if the node is active.
func (s *Streamlet) handleBundle(bm BundleMessage) {
	// All nodes gossip all received bundles, even if inactive.
	// Processing and voting is done only if active.
	s.gossip.Broadcast(bm)
	if s.isActive() {
		s.processBundle(bm)
	}
}

// processBundle processes a received bundle message.
// It adds the bundle to the local state, checks if it belongs to the longest
// notarized chain, and votes for it if so while updating local state accordingly.
func (s *Streamlet) processBundle(bm BundleMessage) {
	// Ignore bundles from other epochs.
	if bm.Epoch != s.epoch {
		return
	}
	s.addBundle(bm)
	// Get length of the chain the new bundle belongs to.
	chainLength := s.chainLength(bm)
	// If the chain is the longest, vote for the bundle and set it as longest.
	if chainLength > s.longestNotarizedChainsLength {
		s.longestNotarizedChains = []types.Hash{bm.Hash()}
		s.longestNotarizedChainsLength = chainLength
		// Vote by sending a bundle message with own ID as voter.
		voteBundle := bm
		voteBundle.Voter = s.config.SelfId
		s.gossip.Broadcast(voteBundle)
	} else if chainLength == s.longestNotarizedChainsLength {
		// If the chain is tied for longest, add it to the list of longest chains.
		s.longestNotarizedChains = append(s.longestNotarizedChains, bm.Hash())
	}

}

// addBundle adds a bundle message to the local state,
// initializes its vote counter if not present, and checks
// if it can be finalized.
func (s *Streamlet) addBundle(bm BundleMessage) {
	voter := bm.Voter
	bm.Voter = model.CreatorId(0) // No voter info in hashToBundle.
	// Store the bundle.
	s.hashToBundle[bm.Hash()] = bm
	// Initialize vote counter if not present.
	if _, exists := s.votesForBundles[bm.Hash()]; !exists {
		s.votesForBundles[bm.Hash()] = consensus.NewVoteCounter(&s.config.Committee)
	}
	// Add the vote from the sender. Error ignored as receiving a message from
	// a non-committee member should be ignored.
	_ = s.votesForBundles[bm.Hash()].Vote(voter)

	// Check if bundles can be finalized.
	// If there are three consecutive bundles with consecutive epochs in a notarized chain,
	// the whole subchain can be finalized, except the latest bundle.
	latestThreeBundles := []BundleMessage{bm, {}, {}}
	latestThreeBundles[1] = s.hashToBundle[bm.LastBundleHash]
	latestThreeBundles[2] = s.hashToBundle[latestThreeBundles[1].LastBundleHash]
	if latestThreeBundles[0].Epoch == latestThreeBundles[1].Epoch+1 &&
		latestThreeBundles[1].Epoch == latestThreeBundles[2].Epoch+1 {
		chainIsNotarized := true
		curBundle := bm
		for {
			// Found an unnotarized bundle, stop.
			if !s.isNotarized(curBundle.Hash()) {
				chainIsNotarized = false
				break
			}
			// Reached a finalized bundle, subchain is notarized.
			// Genesis is always finalized, meaning this will always terminate.
			if _, isFinalized := s.finalizedBundles[curBundle.Hash()]; isFinalized {
				break
			}
			curBundle = s.hashToBundle[curBundle.LastBundleHash]
		}
		if chainIsNotarized {
			s.finalizeBundle(latestThreeBundles[0].Hash())
		}
	}

}

// finalizeBundle finalizes the bundle with the given hash
// and recursively finalizes its ancestors if they are not already finalized.
func (s *Streamlet) finalizeBundle(hash types.Hash) {
	if _, alreadyFinalized := s.finalizedBundles[hash]; alreadyFinalized {
		return
	}
	s.finalizedBundles[hash] = struct{}{}
	slog.Info("Finalized bundle", "creator", s.config.SelfId, "hash", hash)
	prevBundle, exists := s.hashToBundle[hash]
	if exists {
		if _, isFinalized := s.finalizedBundles[prevBundle.LastBundleHash]; !isFinalized {
			s.finalizeBundle(prevBundle.Hash())
		}
	}
	s.notifyListeners(s.hashToBundle[hash].Bundle)
}

// isNotarized checks if a bundle with the given hash has reached quorum.
func (s *Streamlet) isNotarized(hash types.Hash) bool {
	return s.votesForBundles[hash].IsQuorumReached()
}

// chainLength recursively computes the length of the chain
// ending with this bundle message. It stops when it reaches a genesis bundle.
func (s *Streamlet) chainLength(bm BundleMessage) int {
	if bm.LastBundleHash == (types.Hash{}) {
		return 1
	}
	parent, exists := s.hashToBundle[bm.LastBundleHash]
	if !exists {
		return 1
	}
	return 1 + s.chainLength(parent)
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

func (a *onMessageAdapter) OnMessage(bm BundleMessage) {
	a.streamlet.handleBundle(bm)
}

// BundleMessage represents a message containing a bundle and its metadata.
// It includes the epoch, the bundle itself, the hash of the last bundle,
// and the ID of the voter who sent the message (as every message
// is essentially a vote).
type BundleMessage struct {
	Epoch          int
	Bundle         types.Bundle
	LastBundleHash types.Hash
	Voter          model.CreatorId
}

// Hash computes a simple hash of the BundleMessage for identification purposes.
// It does not take Voter into account for the hash.
func (bm BundleMessage) Hash() types.Hash {
	data := fmt.Sprintf("%+v%+v%+v", bm.Epoch, bm.Bundle, bm.LastBundleHash)
	return types.Sha256([]byte(data))
}

// HashWithVoter computes a hash of the BundleMessage including the Voter field.
// This is useful for distinguishing messages from different voters, for gossiping.
func (bm BundleMessage) HashWithVoter() types.Hash {
	data := fmt.Sprintf("%+v", bm)
	return types.Sha256([]byte(data))
}
