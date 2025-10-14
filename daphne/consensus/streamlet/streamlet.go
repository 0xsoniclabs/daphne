package streamlet

import (
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
	return NewPassiveStreamlet(p2pServer, &f)
}

// NewActive creates a new active Streamlet consensus instance.
func (f Factory) NewActive(p2pServer p2p.Server,
	source consensus.TransactionProvider) consensus.Consensus {
	return NewActiveStreamlet(p2pServer, source, &f)
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

	gossip generic.Gossip[BundleMessage]
}

func NewPassiveStreamlet(
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
	genesisBundle := BundleMessage{}
	// Create genesis bundle.
	res.addBundle(genesisBundle)
	// Notarize genesis bundle.
	for _, creator := range config.Committee.Creators() {
		res.votesForBundles[genesisBundle.Hash()].Vote(creator)
	}
	res.longestNotarizedChains = []types.Hash{genesisBundle.Hash()}
	res.longestNotarizedChainsLength = 1
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

func (s *Streamlet) RegisterListener(listener consensus.BundleListener) {
	s.listenersMutex.Lock()
	defer s.listenersMutex.Unlock()
	s.listeners = append(s.listeners, listener)
}

func (s *Streamlet) addBundle(bm BundleMessage) {
	bm.Voter = 0
	s.hashToBundle[bm.Hash()] = bm
	if _, exists := s.votesForBundles[bm.Hash()]; !exists {
		s.votesForBundles[bm.Hash()] = consensus.NewVoteCounter(&s.config.Committee)
	}
}

func (s *Streamlet) isNotarized(hash types.Hash) bool {
	return s.votesForBundles[hash].IsQuorumReached()
}

func (s *Streamlet) getLeader() model.CreatorId {
	creators := s.config.Committee.Creators()
	return creators[s.epoch%len(creators)]
}

func (s *Streamlet) handleBundle(bm BundleMessage) {
	//TODO: implement
}

func (s *Streamlet) isActive() bool {
	return slices.Contains(s.config.Committee.Creators(), s.config.SelfId)
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
	return types.Hash([]byte(data))
}

// HashWithVoter computes a hash of the BundleMessage including the Voter field.
// This is useful for distinguishing messages from different voters, for gossiping.
func (bm BundleMessage) HashWithVoter() types.Hash {
	data := fmt.Sprintf("%+v", bm)
	return types.Hash([]byte(data))
}

// ChainLength recursively computes the length of the chain
// ending with this bundle message. It stops when it reaches a genesis bundle.
func (bm BundleMessage) ChainLength(bundleMap map[types.Hash]BundleMessage) int {
	if bm.LastBundleHash == (types.Hash{}) {
		return 1
	}
	parent, exists := bundleMap[bm.LastBundleHash]
	if !exists {
		return 1
	}
	return 1 + parent.ChainLength(bundleMap)
}
