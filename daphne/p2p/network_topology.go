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

package p2p

import (
	"fmt"
	"maps"
	"math/rand"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

//go:generate mockgen -source network_topology.go -destination=network_topology_mock.go -package=p2p

// NetworkTopology defines the connectivity graph between peers in the network.
// It determines whether a direct connection should exist between any two peers.
type NetworkTopology interface {
	// ShouldConnect determines if a directed connection should be established
	// from one peer to another.
	ShouldConnect(from, to PeerId) bool
}

// TopologyFactory defines the methods required to instantiate a network topology.
type TopologyFactory interface {
	// Create creates a new network topology for the given set of peers.
	// It takes a mapping of peers to layers, which can be used by certain
	// topology implementations to define layer-based connectivity rules.
	// The returned NetworkTopology will be used to determine connections
	// between the provided peers.
	Create(peers map[PeerId]int) NetworkTopology
	// Stringer is required to make factories usable in logging and reporting.
	fmt.Stringer
}

// --- FullyMeshedTopology ---

// FullyMeshedTopology implements a topology where every peer is
// connected to every other peer, forming a complete graph.
type FullyMeshedTopology struct{}

// NewFullyMeshedTopology creates a new fully connected topology.
func NewFullyMeshedTopology() *FullyMeshedTopology {
	return &FullyMeshedTopology{}
}

// ShouldConnect in a fully connected topology always returns true, establishing
// a bidirectional link since ShouldConnect(a, b) and ShouldConnect(b, a)
// will both be true for all peers a, b. Peers are not connected to themselves
// in this implementation.
func (t *FullyMeshedTopology) ShouldConnect(from, to PeerId) bool {
	return from != to
}

// String returns a string representation of the topology.
func (t *FullyMeshedTopology) String() string {
	return "fully-meshed"
}

// FullyMeshedTopologyFactory is a factory for creating fully meshed topologies.
type FullyMeshedTopologyFactory struct{}

// Create creates a new fully meshed topology for the given peers.
// The layer information is ignored as all peers are treated equally in a fully meshed topology.
func (f FullyMeshedTopologyFactory) Create(peers map[PeerId]int) NetworkTopology {
	return NewFullyMeshedTopology()
}

// String returns a string representation of the factory.
func (f FullyMeshedTopologyFactory) String() string {
	return "fully-meshed"
}

// --- LineTopology ---

// LineTopology implements a topology where each peer is connected to its
// adjacent neighbors, forming a line. The first and last peers will only
// have one connection each. This implementation requires the complete set
// of peers to be known at its creation.
type LineTopology struct {
	peerIndex map[PeerId]int
}

// NewLineTopology creates a new line topology from an ordered list of peers.
// The order of peers in the provided slice determines their position and
// neighbors in the line.
func NewLineTopology(peers []PeerId) *LineTopology {
	uniquePeers := sets.New(peers...).ToSlice()
	slices.Sort(uniquePeers)
	peerIndex := make(map[PeerId]int, len(uniquePeers))
	for i, p := range uniquePeers {
		peerIndex[p] = i
	}
	return &LineTopology{
		peerIndex: peerIndex,
	}
}

// ShouldConnect determines if two peers are adjacent in the line. A peer will
// connect to its predecessor and successor in the initial list. The connection
// is bidirectional. Unlike the RingTopology, the ends of the line do not
// connect to each other.
func (t *LineTopology) ShouldConnect(from, to PeerId) bool {
	fromIndex, fromExists := t.peerIndex[from]
	toIndex, toExists := t.peerIndex[to]

	// Both peers must be part of the predefined line topology.
	if !fromExists || !toExists {
		return false
	}

	// Peers do not connect to themselves.
	if from == to {
		return false
	}

	// Check if 'to' is the peer immediately after 'from' in the line.
	if toIndex == fromIndex+1 {
		return true
	}

	// Check if 'to' is the peer immediately before 'from' in the line.
	if toIndex == fromIndex-1 {
		return true
	}

	return false
}

// String returns a string representation of the topology.
func (t *LineTopology) String() string {
	return fmt.Sprintf("line-%d", len(t.peerIndex))
}

// LineTopologyFactory is a factory for creating line topologies.
type LineTopologyFactory struct{}

// Create creates a new line topology for the given peers.
// The layer information is ignored as all peers are treated as a single layer.
func (f LineTopologyFactory) Create(peers map[PeerId]int) NetworkTopology {
	return NewLineTopology(slices.Collect(maps.Keys(peers)))
}

// String returns a string representation of the factory.
func (f LineTopologyFactory) String() string {
	return "line"
}

// --- RingTopology ---

// RingTopology implements a topology where each peer is connected to its two
// adjacent neighbors, forming a ring. This implementation requires the complete
// set of peers to be known at the time of its creation.
type RingTopology struct {
	peerIndex map[PeerId]int
}

// NewRingTopology creates a new ring topology from an ordered list of peers.
// The order of peers in the provided slice determines their position and
// neighbors in the ring.
func NewRingTopology(peers []PeerId) *RingTopology {
	uniquePeers := sets.New(peers...).ToSlice()
	slices.Sort(uniquePeers)
	peerIndex := make(map[PeerId]int, len(uniquePeers))
	for i, p := range uniquePeers {
		peerIndex[p] = i
	}
	return &RingTopology{
		peerIndex: peerIndex,
	}
}

// ShouldConnect determines if two peers are adjacent in the ring. A peer will
// connect to its predecessor and successor in the initial list. The connection
// is bidirectional.
func (t *RingTopology) ShouldConnect(from, to PeerId) bool {
	fromIndex, fromExists := t.peerIndex[from]
	toIndex, toExists := t.peerIndex[to]

	// Both peers must be part of the predefined ring topology.
	if !fromExists || !toExists {
		return false
	}

	// Peers do not connect to themselves.
	if from == to {
		return false
	}

	// Check if 'to' is the peer immediately after 'from' in the ring.
	if toIndex == (fromIndex+1)%len(t.peerIndex) {
		return true
	}

	// Check if 'to' is the peer immediately before 'from' in the ring.
	if toIndex == ((fromIndex - 1 + len(t.peerIndex)) % len(t.peerIndex)) {
		return true
	}

	return false
}

// String returns a string representation of the topology.
func (t *RingTopology) String() string {
	return fmt.Sprintf("ring-%d", len(t.peerIndex))
}

// RingTopologyFactory is a factory for creating ring topologies.
type RingTopologyFactory struct{}

// Create creates a new ring topology for the given peers.
// The layer information is ignored as all peers are treated as a single layer.
func (f RingTopologyFactory) Create(peers map[PeerId]int) NetworkTopology {
	return NewRingTopology(slices.Collect(maps.Keys(peers)))
}

// String returns a string representation of the factory.
func (f RingTopologyFactory) String() string {
	return "ring"
}

// --- StarTopology ---

// StarTopology implements a topology where one peer acts as a central hub,
// and all other peers (spokes) connect only to this hub.
type StarTopology struct {
	hub   PeerId
	peers map[PeerId]bool // Store all valid peers for quick lookup.
}

// NewStarTopology creates a new star topology.
// It requires the ID of the peer that will act as the central hub. The hub
// peer must be present in the provided list of all peers, otherwise this
// function will panic.
func NewStarTopology(hub PeerId, peers []PeerId) *StarTopology {
	peerSet := make(map[PeerId]bool, len(peers))
	for _, p := range peers {
		peerSet[p] = true
	}

	// Check that the hub is in the provided peer set.
	if !peerSet[hub] {
		panic(fmt.Sprintf("hub PeerId '%v' must be present in the peers slice", hub))
	}

	return &StarTopology{
		hub:   hub,
		peers: peerSet,
	}
}

// ShouldConnect determines if a connection should be made in a star topology.
// A connection is established if and only if one of the peers is the hub,
// the other is a spoke, and both are part of the defined topology.
// Spokes do not connect to other spokes.
func (t *StarTopology) ShouldConnect(from, to PeerId) bool {
	if !t.peers[from] || !t.peers[to] {
		return false
	}

	// Peers do not connect to themselves.
	if from == to {
		return false
	}

	// A connection exists if one peer is the hub and the other is not.
	// This implicitly handles both hub-to-spoke and spoke-to-hub connections.
	return from == t.hub || to == t.hub
}

// String returns a string representation of the topology.
func (t *StarTopology) String() string {
	return fmt.Sprintf("star-%d", len(t.peers))
}

// StarTopologyFactory is a factory for creating star topologies.
type StarTopologyFactory struct{}

// Create creates a new star topology for the given peers.
// The layer information is ignored as all peers are treated as a single layer.
// The first peer (lexicographically by PeerId) is used as the hub.
func (f StarTopologyFactory) Create(peers map[PeerId]int) NetworkTopology {
	if len(peers) == 0 {
		panic("cannot create star topology with no peers")
	}
	peerList := slices.Sorted(maps.Keys(peers))
	return NewStarTopology(peerList[0], peerList)
}

// String returns a string representation of the factory.
func (f StarTopologyFactory) String() string {
	return "star"
}

// --- RandomNaryGraphTopology ---

// RandomNaryGraphTopology implements a topology where each peer connects up to
// n other peers.
//
// The graph is constructed by first connecting adjacent neighbors to form a
// ring, which ensures baseline connectivity (for n >= 2). Additional random
// connections are then added until each peer has up to n connections,
// stopping when no more valid connections can be formed.
//
// Note on the Handshake Lemma: An odd number of peers cannot all have an
// odd number of connections.
type RandomNaryGraphTopology struct {
	// connections stores the pre-calculated adjacency list for the graph.
	connections map[PeerId]map[PeerId]bool
	// n is the desired number of connections for each peer.
	n int
	// seed is the random seed used for graph generation.
	seed int64
}

// NewRandomNaryGraphTopology creates a new random n-ary graph topology.
//
// Parameters:
//   - peers: The complete list of peers in the network. Their order determines
//     the initial neighbor connections.
//   - n: The desired number of connections for each peer.
//   - seed: A seed for the random number generator to ensure determinism.
func NewRandomNaryGraphTopology(
	peers []PeerId,
	n int,
	seed int64,
) *RandomNaryGraphTopology {
	uniquePeers := sets.New(peers...).ToSlice()
	slices.Sort(uniquePeers)
	rng := rand.New(rand.NewSource(seed))
	numPeers := len(uniquePeers)

	// Clamp n to a valid range [0, num_peers - 1].
	if n < 0 {
		n = 0
	}
	if numPeers > 1 && n >= numPeers {
		n = numPeers - 1
	}

	connections := make(map[PeerId]map[PeerId]bool)
	numConnections := make(map[PeerId]int)
	for _, p := range uniquePeers {
		connections[p] = make(map[PeerId]bool)
		numConnections[p] = 0
	}

	// --- Phase 1: Establish a Ring Topology as a baseline ---
	// This ensures the graph is connected from the start (if n >= 2).
	if numPeers > 1 {
		for i, p1 := range uniquePeers {
			// Connect to the next peer in the ring. This single loop
			// creates a bidirectional ring because when we process p2 later,
			// its next peer will be p3, and so on.
			p2 := uniquePeers[(i+1)%numPeers]
			if numConnections[p1] < n && numConnections[p2] < n {
				connections[p1][p2] = true
				connections[p2][p1] = true
				numConnections[p1]++
				numConnections[p2]++
			}
		}
	}

	// --- Phase 2: Add additional random connections ---
	// Create a mutable copy of the peer list for shuffling.
	peerList := slices.Clone(uniquePeers)

	// Iterate through each peer to find additional random partners.
	for _, p1 := range peerList {
		// Re-shuffle for every peer to ensure fairness and prevent ordering bias.
		rng.Shuffle(len(peerList), func(i, j int) {
			peerList[i], peerList[j] = peerList[j], peerList[i]
		})

		// Find new peers to connect to.
		for _, p2 := range peerList {
			// Stop if p1 has reached its desired number of connections.
			if numConnections[p1] == n {
				break
			}

			// A new random connection is made if:
			// - They are not the same peer.
			// - The target peer (p2) ALSO has capacity for another connection.
			// - They are not already connected (either from Phase 1 or
			// earlier in Phase 2).
			if p1 != p2 && numConnections[p2] < n && !connections[p1][p2] {
				connections[p1][p2] = true
				connections[p2][p1] = true
				numConnections[p1]++
				numConnections[p2]++
			}
		}
	}

	return &RandomNaryGraphTopology{
		connections: connections,
		n:           n,
		seed:        seed,
	}
}

// ShouldConnect determines if a connection exists between two peers by checking
// the pre-calculated graph.
func (t *RandomNaryGraphTopology) ShouldConnect(from, to PeerId) bool {
	if connectionsTo, fromExists := t.connections[from]; fromExists {
		return connectionsTo[to]
	}
	return false
}

// String returns a string representation of the topology.
func (t *RandomNaryGraphTopology) String() string {
	return fmt.Sprintf("random-%d-seed%d", t.n, t.seed)
}

// RandomNaryGraphTopologyFactory is a factory for creating random n-ary graph topologies.
type RandomNaryGraphTopologyFactory struct {
	// N is the desired number of connections for each peer.
	N int
	// Seed is the random seed for deterministic graph generation.
	Seed int64
}

// Create creates a new random n-ary graph topology for the given peers.
// The layer information is ignored as all peers are treated as a single layer.
func (f RandomNaryGraphTopologyFactory) Create(peers map[PeerId]int) NetworkTopology {
	return NewRandomNaryGraphTopology(slices.Sorted(maps.Keys(peers)), f.N, f.Seed)
}

// String returns a string representation of the factory.
func (f RandomNaryGraphTopologyFactory) String() string {
	return fmt.Sprintf("random-%d-seed%d", f.N, f.Seed)
}

// --- TwoLayerTopology ---

// TwoLayerTopology implements a topology where peers are divided into two layers:
// validators and non-validators. Validators connect to all other validators. Non-validators
// connect to one random validator.

type TwoLayerTopology struct {
	validators              []PeerId
	nonValidators           []PeerId
	nonValidatorConnections map[PeerId]PeerId
}

const (
	TwoLayerValidatorLayer = iota
	TwoLayerNonValidatorLayer
)

// NewTwoLayerTopology creates a new two-layer topology from the given peers and their layers.
func NewTwoLayerTopology(peers map[PeerId]int, seed int64) *TwoLayerTopology {
	var validators []PeerId
	var nonValidators []PeerId
	for p, layer := range peers {
		if layer == TwoLayerValidatorLayer {
			validators = append(validators, p)
		} else {
			nonValidators = append(nonValidators, p)
		}
	}

	// Sort for deterministic behavior
	slices.Sort(validators)
	slices.Sort(nonValidators)

	nonValidatorConnections := make(map[PeerId]PeerId)
	if len(validators) > 0 {
		rng := rand.New(rand.NewSource(seed))
		for _, p := range nonValidators {
			nonValidatorConnections[p] = validators[rng.Intn(len(validators))]
		}
	}

	return &TwoLayerTopology{
		validators:              validators,
		nonValidators:           nonValidators,
		nonValidatorConnections: nonValidatorConnections,
	}
}

// ShouldConnect determines if a connection should be made in the two-layer topology.
func (t *TwoLayerTopology) ShouldConnect(from, to PeerId) bool {
	// Peers do not connect to themselves
	if from == to {
		return false
	}

	fromIsValidator := slices.Contains(t.validators, from)
	toIsValidator := slices.Contains(t.validators, to)

	// Validator to Validator: connect
	if fromIsValidator && toIsValidator {
		return true
	}

	return t.nonValidatorConnections[from] == to || t.nonValidatorConnections[to] == from
}

// String returns a string representation of the topology.
func (t *TwoLayerTopology) String() string {
	return fmt.Sprintf("two-layer-v%d-n%d", len(t.validators), len(t.nonValidators))
}

// TwoLayerTopologyFactory is a factory for creating two-layer topologies.
type TwoLayerTopologyFactory struct {
	// Seed is the random seed for deterministic assignment of non-validators to validators.
	Seed int64
}

// Create creates a new two-layer topology for the given peers.
func (f TwoLayerTopologyFactory) Create(peers map[PeerId]int) NetworkTopology {
	return NewTwoLayerTopology(peers, f.Seed)
}

// String returns a string representation of the factory.
func (f TwoLayerTopologyFactory) String() string {
	return fmt.Sprintf("two-layer-seed%d", f.Seed)
}
