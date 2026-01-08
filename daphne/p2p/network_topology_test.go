package p2p

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFullyMeshedTopology_ShouldConnect_ReturnsTrueForAllPeersOtherThanItself(t *testing.T) {
	topology := NewFullyMeshedTopology()

	peerA := PeerId("peer-A")
	peerB := PeerId("peer-B")
	peerC := PeerId("peer-C")

	require.True(
		t,
		topology.ShouldConnect(peerA, peerB),
		"should connect peer A to peer B",
	)
	require.True(
		t,
		topology.ShouldConnect(peerB, peerA),
		"should connect peer B to peer A",
	)
	require.True(
		t,
		topology.ShouldConnect(peerA, peerC),
		"should connect peer A to peer C",
	)
	require.False(
		t,
		topology.ShouldConnect(peerA, peerA),
		"should allow a peer to connect to itself",
	)
}

func TestLineTopology_ShouldConnect_ReturnsTrueOnlyForNeighbors(t *testing.T) {
	peerA := PeerId("peer-A")
	peerB := PeerId("peer-B")
	peerC := PeerId("peer-C")
	peerD := PeerId("peer-D")
	peerZ := PeerId("peer-Z") // A peer not in the line

	tests := map[string]struct {
		linePeers []PeerId
		from      PeerId
		to        PeerId
		want      bool
	}{
		"4 peers: connects adjacent A to B": {
			linePeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerA,
			to:        peerB,
			want:      true,
		},
		"4 peers: connects adjacent B to A": {
			linePeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerB,
			to:        peerA,
			want:      true,
		},
		"4 peers: does NOT connect wrap-around D to A": {
			linePeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerD,
			to:        peerA,
			want:      false,
		},
		"4 peers: does not connect non-adjacent A to C": {
			linePeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerA,
			to:        peerC,
			want:      false,
		},
		"4 peers: does not connect to self": {
			linePeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerA,
			to:        peerA,
			want:      false,
		},
		"4 peers: does not connect to peer not in line": {
			linePeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerA,
			to:        peerZ,
			want:      false,
		},
		"4 peers: does not connect from peer not in line": {
			linePeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerZ,
			to:        peerA,
			want:      false,
		},
		"2 peers: connects A to B": {
			linePeers: []PeerId{peerA, peerB},
			from:      peerA,
			to:        peerB,
			want:      true,
		},
		"1 peer: does not connect to self": {
			linePeers: []PeerId{peerA},
			from:      peerA,
			to:        peerA,
			want:      false,
		},
		"0 peers: does not connect anything": {
			linePeers: []PeerId{},
			from:      peerA,
			to:        peerB,
			want:      false,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			topology := NewLineTopology(testCase.linePeers)
			got := topology.ShouldConnect(testCase.from, testCase.to)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestStarTopology_ShouldConnect_ReturnsTrueOnlyForHub(t *testing.T) {
	hub := PeerId("hub")
	spokeA := PeerId("spoke-A")
	spokeB := PeerId("spoke-B")
	spokeC := PeerId("spoke-C")
	peerZ := PeerId("peer-Z") // A peer not in the star

	allPeers := []PeerId{hub, spokeA, spokeB, spokeC}

	tests := map[string]struct {
		from PeerId
		to   PeerId
		want bool
	}{
		"connects hub to spoke": {
			from: hub,
			to:   spokeA,
			want: true,
		},
		"connects spoke to hub": {
			from: spokeB,
			to:   hub,
			want: true,
		},
		"does not connect spoke to spoke": {
			from: spokeA,
			to:   spokeB,
			want: false,
		},
		"does not connect hub to self": {
			from: hub,
			to:   hub,
			want: false,
		},
		"does not connect spoke to self": {
			from: spokeA,
			to:   spokeA,
			want: false,
		},
		"does not connect to peer not in star": {
			from: hub,
			to:   peerZ,
			want: false,
		},
		"does not connect from peer not in star": {
			from: peerZ,
			to:   hub,
			want: false,
		},
	}

	topology := NewStarTopology(hub, allPeers)

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			// Hub is present, so this should not panic.
			got := topology.ShouldConnect(testCase.from, testCase.to)
			require.Equal(t, testCase.want, got)
		})
	}

	t.Run("panics if hub is not in peer list", func(t *testing.T) {
		spokesOnly := []PeerId{spokeA, spokeB, spokeC}
		require.Panics(t, func() {
			NewStarTopology(hub, spokesOnly)
		})
	})
}

func TestRingTopology_ShouldConnect_ReturnsTrueOnlyForNeighbors(t *testing.T) {
	peerA := PeerId("peer-A")
	peerB := PeerId("peer-B")
	peerC := PeerId("peer-C")
	peerD := PeerId("peer-D")
	peerZ := PeerId("peer-Z") // A peer not in the ring

	tests := map[string]struct {
		ringPeers []PeerId
		from      PeerId
		to        PeerId
		want      bool
	}{
		"4 peers: connects adjacent A to B": {
			ringPeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerA,
			to:        peerB,
			want:      true,
		},
		"4 peers: connects adjacent B to A": {
			ringPeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerB,
			to:        peerA,
			want:      true,
		},
		"4 peers: connects wrap-around D to A": {
			ringPeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerD,
			to:        peerA,
			want:      true,
		},
		"4 peers: does not connect non-adjacent A to C": {
			ringPeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerA,
			to:        peerC,
			want:      false,
		},
		"4 peers: does not connect to self": {
			ringPeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerA,
			to:        peerA,
			want:      false,
		},
		"4 peers: does not connect to peer not in ring": {
			ringPeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerA,
			to:        peerZ,
			want:      false,
		},
		"4 peers: does not connect from peer not in ring": {
			ringPeers: []PeerId{peerA, peerB, peerC, peerD},
			from:      peerZ,
			to:        peerA,
			want:      false,
		},
		"2 peers: connects A to B": {
			ringPeers: []PeerId{peerA, peerB},
			from:      peerA,
			to:        peerB,
			want:      true,
		},
		"2 peers: connects B to A": {
			ringPeers: []PeerId{peerA, peerB},
			from:      peerB,
			to:        peerA,
			want:      true,
		},
		"1 peer: does not connect to self": {
			ringPeers: []PeerId{peerA},
			from:      peerA,
			to:        peerA,
			want:      false,
		},
		"0 peers: does not connect anything": {
			ringPeers: []PeerId{},
			from:      peerA,
			to:        peerB,
			want:      false,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			topology := NewRingTopology(testCase.ringPeers)
			got := topology.ShouldConnect(testCase.from, testCase.to)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestTwoLayerTopology_ShouldConnect_ValidatorsFullyMeshedNonValidatorsToValidator(t *testing.T) {
	v1 := PeerId("validator-1")
	v2 := PeerId("validator-2")
	v3 := PeerId("validator-3")
	nv1 := PeerId("non-validator-1")
	nv2 := PeerId("non-validator-2")
	nv3 := PeerId("non-validator-3")
	peerZ := PeerId("peer-Z") // A peer not in the topology

	allPeers := map[PeerId]int{
		v1:  TwoLayerValidatorLayer,
		v2:  TwoLayerValidatorLayer,
		v3:  TwoLayerValidatorLayer,
		nv1: TwoLayerNonValidatorLayer,
		nv2: TwoLayerNonValidatorLayer,
		nv3: TwoLayerNonValidatorLayer,
	}

	tests := map[string]struct {
		from PeerId
		to   PeerId
		want bool
	}{
		"connects validator to validator": {
			from: v1,
			to:   v2,
			want: true,
		},
		"connects validator to validator bidirectional": {
			from: v2,
			to:   v1,
			want: true,
		},
		"connects all validators v1 to v3": {
			from: v1,
			to:   v3,
			want: true,
		},
		"connects all validators v3 to v2": {
			from: v3,
			to:   v2,
			want: true,
		},
		"does not connect validator to self": {
			from: v1,
			to:   v1,
			want: false,
		},
		"does not connect non-validator to self": {
			from: nv1,
			to:   nv1,
			want: false,
		},
		"does not connect to peer not in topology": {
			from: v1,
			to:   peerZ,
			want: false,
		},
		"does not connect from peer not in topology": {
			from: peerZ,
			to:   v1,
			want: false,
		},
		"does not connect non-validators to each other": {
			from: nv1,
			to:   nv2,
			want: false,
		},
	}

	topology := NewTwoLayerTopology(allPeers, 42)

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			got := topology.ShouldConnect(testCase.from, testCase.to)
			require.Equal(t, testCase.want, got)
		})
	}

	t.Run("each non-validator connects to exactly one validator", func(t *testing.T) {
		nonValidators := []PeerId{nv1, nv2, nv3}
		validators := []PeerId{v1, v2, v3}

		for _, nv := range nonValidators {
			connectionCount := 0
			var connectedValidator PeerId
			for _, v := range validators {
				if topology.ShouldConnect(nv, v) {
					connectionCount++
					connectedValidator = v
				}
			}
			require.Equal(
				t,
				1,
				connectionCount,
				"non-validator %s should connect to exactly one validator",
				nv,
			)

			// Verify bidirectional connection
			require.True(
				t,
				topology.ShouldConnect(connectedValidator, nv),
				"connection should be bidirectional from %s to %s",
				connectedValidator,
				nv,
			)
		}
	})
}

func TestRandomNaryGraphTopology_NewRandomNaryGraphTopology_CreatesTopologyWithExpectedProperties(t *testing.T) {
	peers := make([]PeerId, 50)
	for i := range peers {
		peers[i] = PeerId(fmt.Sprintf("peer-%d", i))
	}
	n := 8
	seed := int64(99)

	topology := NewRandomNaryGraphTopology(peers, n, seed)

	require.Equal(
		t,
		len(topology.connections),
		len(peers),
		"should have one entry for each peer",
	)

	for from, connections := range topology.connections {
		connectionCount := len(connections)

		require.LessOrEqual(
			t,
			connectionCount,
			n,
			"peer %s has %d connections, want at most %d",
			from,
			connectionCount,
			n,
		)
		require.GreaterOrEqual(
			t,
			connectionCount,
			2,
			"peer %s has %d connections, want at minimum 2",
			from,
			connectionCount,
			n,
		)
		require.False(
			t,
			topology.ShouldConnect(from, from),
			"peer %s should not connect to itself",
			from,
		)
		for to := range connections {
			require.True(
				t,
				topology.ShouldConnect(to, from),
				"connection should be bidirectional, but %s does not connect back to %s",
				to,
				from,
			)
			require.True(
				t,
				topology.ShouldConnect(from, to),
				"connection should be bidirectional, but %s does not connect back to %s",
				to,
				from,
			)
		}
	}
}

func TestRandomNaryGraphTopology_EdgeCases(t *testing.T) {
	peers := []PeerId{"A", "B", "C"}
	peerA, peerB, peerC := peers[0], peers[1], peers[2]

	tests := map[string]struct {
		peers []PeerId
		n     int
		from  PeerId
		to    PeerId
		want  bool
	}{
		"n=0: no connections": {
			peers: peers,
			n:     0,
			from:  peerA,
			to:    peerB,
			want:  false,
		},
		"n is negative: clamped to 0, no connections": {
			peers: peers,
			n:     -5,
			from:  peerA,
			to:    peerB,
			want:  false,
		},
		"n >= num_peers: clamped to num_peers-1, becomes fully meshed": {
			peers: peers,
			n:     3, // >= 3 peers
			from:  peerA,
			to:    peerB,
			want:  true,
		},
		"n > num_peers: clamped to num_peers-1, becomes fully meshed": {
			peers: peers,
			n:     100, // > 3 peers
			from:  peerA,
			to:    peerC,
			want:  true,
		},
		"n >= num_peers: does not connect to self": {
			peers: peers,
			n:     3,
			from:  peerA,
			to:    peerA,
			want:  false,
		},
		"empty peer list": {
			peers: []PeerId{},
			n:     2,
			from:  peerA,
			to:    peerB,
			want:  false,
		},
		"single peer list": {
			peers: []PeerId{peerA},
			n:     1,
			from:  peerA,
			to:    peerA,
			want:  false,
		},
		"peer not in list": {
			peers: []PeerId{peerA, peerB},
			n:     1,
			from:  peerA,
			to:    "unknown-peer",
			want:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			topology := NewRandomNaryGraphTopology(tc.peers, tc.n, 1)
			got := topology.ShouldConnect(tc.from, tc.to)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestTwoLayerTopology_EdgeCases(t *testing.T) {
	tests := map[string]struct {
		peers map[PeerId]int
		seed  int64
		from  PeerId
		to    PeerId
		want  bool
	}{
		"only validators connect to each other": {
			peers: map[PeerId]int{
				"v1": TwoLayerValidatorLayer,
				"v2": TwoLayerValidatorLayer,
			},
			seed: 1,
			from: "v1",
			to:   "v2",
			want: true,
		},
		"single validator does not connect to self": {
			peers: map[PeerId]int{
				"v1": TwoLayerValidatorLayer,
			},
			seed: 1,
			from: "v1",
			to:   "v1",
			want: false,
		},
		"only non-validators with no validators do not connect": {
			peers: map[PeerId]int{
				"nv1": TwoLayerNonValidatorLayer,
				"nv2": TwoLayerNonValidatorLayer,
			},
			seed: 1,
			from: "nv1",
			to:   "nv2",
			want: false,
		},
		"single validator and single non-validator connect": {
			peers: map[PeerId]int{
				"v1":  TwoLayerValidatorLayer,
				"nv1": TwoLayerNonValidatorLayer,
			},
			seed: 1,
			from: "v1",
			to:   "nv1",
			want: true,
		},
		"single validator and single non-validator connect bidirectionally": {
			peers: map[PeerId]int{
				"v1":  TwoLayerValidatorLayer,
				"nv1": TwoLayerNonValidatorLayer,
			},
			seed: 1,
			from: "nv1",
			to:   "v1",
			want: true,
		},
		"empty peer list does not connect": {
			peers: map[PeerId]int{},
			seed:  1,
			from:  "any",
			to:    "peer",
			want:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			topology := NewTwoLayerTopology(tc.peers, tc.seed)
			got := topology.ShouldConnect(tc.from, tc.to)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestTwoLayerTopology_NewTwoLayerTopology_IsDeterministicWithSeed(t *testing.T) {
	peers := map[PeerId]int{
		"v1":  TwoLayerValidatorLayer,
		"v2":  TwoLayerValidatorLayer,
		"nv1": TwoLayerNonValidatorLayer,
		"nv2": TwoLayerNonValidatorLayer,
		"nv3": TwoLayerNonValidatorLayer,
	}

	// Create two topologies with the same seed
	topology1 := NewTwoLayerTopology(peers, 42)
	topology2 := NewTwoLayerTopology(peers, 42)

	// Verify they produce the same connections
	allPeers := []PeerId{"v1", "v2", "nv1", "nv2", "nv3"}
	for _, from := range allPeers {
		for _, to := range allPeers {
			require.Equal(
				t,
				topology1.ShouldConnect(from, to),
				topology2.ShouldConnect(from, to),
				"topologies with same seed should produce same connections for %s -> %s",
				from,
				to,
			)
		}
	}
}

func TestFullyMeshedTopology_String_ReturnsExpectedFormat(t *testing.T) {
	topology := NewFullyMeshedTopology()
	require.Equal(t, "fully-meshed", topology.String())
}

func TestLineTopology_String_ReturnsExpectedFormat(t *testing.T) {
	tests := map[string]struct {
		peers []PeerId
		want  string
	}{
		"empty peer list": {
			peers: []PeerId{},
			want:  "line-0",
		},
		"single peer": {
			peers: []PeerId{PeerId("peer-A")},
			want:  "line-1",
		},
		"four peers": {
			peers: []PeerId{
				PeerId("peer-A"),
				PeerId("peer-B"),
				PeerId("peer-C"),
				PeerId("peer-D"),
			},
			want: "line-4",
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			topology := NewLineTopology(testCase.peers)
			require.Equal(t, testCase.want, topology.String())
		})
	}
}

func TestRingTopology_String_ReturnsExpectedFormat(t *testing.T) {
	tests := map[string]struct {
		peers []PeerId
		want  string
	}{
		"empty peer list": {
			peers: []PeerId{},
			want:  "ring-0",
		},
		"single peer": {
			peers: []PeerId{PeerId("peer-A")},
			want:  "ring-1",
		},
		"four peers": {
			peers: []PeerId{
				PeerId("peer-A"),
				PeerId("peer-B"),
				PeerId("peer-C"),
				PeerId("peer-D"),
			},
			want: "ring-4",
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			topology := NewRingTopology(testCase.peers)
			require.Equal(t, testCase.want, topology.String())
		})
	}
}

func TestStarTopology_String_ReturnsExpectedFormat(t *testing.T) {
	tests := map[string]struct {
		hub   PeerId
		peers []PeerId
		want  string
	}{
		"hub only": {
			hub:   PeerId("hub"),
			peers: []PeerId{PeerId("hub")},
			want:  "star-1",
		},
		"hub with three spokes": {
			hub: PeerId("hub"),
			peers: []PeerId{
				PeerId("hub"),
				PeerId("spoke-A"),
				PeerId("spoke-B"),
				PeerId("spoke-C"),
			},
			want: "star-4",
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			topology := NewStarTopology(testCase.hub, testCase.peers)
			require.Equal(t, testCase.want, topology.String())
		})
	}
}

func TestRandomNaryGraphTopology_String_ReturnsExpectedFormat(t *testing.T) {
	tests := map[string]struct {
		numPeers int
		n        int
		seed     int64
		want     string
	}{
		"n=3 with 5 peers": {
			numPeers: 5,
			n:        3,
			seed:     42,
			want:     "random-3-seed42",
		},
		"n=5 with 10 peers": {
			numPeers: 10,
			n:        5,
			seed:     100,
			want:     "random-5-seed100",
		},
		"n=0 seed=0": {
			numPeers: 1,
			n:        0,
			seed:     0,
			want:     "random-0-seed0",
		},
		"n clamped to numPeers-1": {
			numPeers: 3,
			n:        10,
			seed:     7,
			want:     "random-2-seed7",
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			peers := make([]PeerId, testCase.numPeers)
			for i := 0; i < testCase.numPeers; i++ {
				peers[i] = PeerId(fmt.Sprintf("peer-%d", i))
			}

			topology := NewRandomNaryGraphTopology(peers, testCase.n, testCase.seed)
			require.Equal(t, testCase.want, topology.String())
		})
	}
}

func TestTwoLayerTopology_String_ReturnsExpectedFormat(t *testing.T) {
	tests := map[string]struct {
		peers map[PeerId]int
		want  string
	}{
		"3 validators and 2 non-validators": {
			peers: map[PeerId]int{
				"v1":  TwoLayerValidatorLayer,
				"v2":  TwoLayerValidatorLayer,
				"v3":  TwoLayerValidatorLayer,
				"nv1": TwoLayerNonValidatorLayer,
				"nv2": TwoLayerNonValidatorLayer,
			},
			want: "two-layer-v3-n2",
		},
		"only validators": {
			peers: map[PeerId]int{
				"v1": TwoLayerValidatorLayer,
				"v2": TwoLayerValidatorLayer,
			},
			want: "two-layer-v2-n0",
		},
		"only non-validators": {
			peers: map[PeerId]int{
				"nv1": TwoLayerNonValidatorLayer,
				"nv2": TwoLayerNonValidatorLayer,
			},
			want: "two-layer-v0-n2",
		},
		"empty peer list": {
			peers: map[PeerId]int{},
			want:  "two-layer-v0-n0",
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			topology := NewTwoLayerTopology(testCase.peers, 1)
			require.Equal(t, testCase.want, topology.String())
		})
	}
}

func TestFullyMeshedTopologyFactory_Create_CreatesTopology(t *testing.T) {
	factory := FullyMeshedTopologyFactory{}
	peers := map[PeerId]int{"A": 0, "B": 0, "C": 0}

	topology := factory.Create(peers)

	require.NotNil(t, topology)
	require.IsType(t, &FullyMeshedTopology{}, topology)
	require.True(t, topology.ShouldConnect("A", "B"))
	require.True(t, topology.ShouldConnect("B", "C"))
	require.False(t, topology.ShouldConnect("A", "A"))
}

func TestFullyMeshedTopologyFactory_String_ReturnsExpectedFormat(t *testing.T) {
	factory := FullyMeshedTopologyFactory{}
	require.Equal(t, "fully-meshed", factory.String())
}

func TestLineTopologyFactory_Create_CreatesTopology(t *testing.T) {
	factory := LineTopologyFactory{}
	peers := map[PeerId]int{"A": 0, "B": 0, "C": 0, "D": 0}

	topology := factory.Create(peers)

	require.NotNil(t, topology)
	require.IsType(t, &LineTopology{}, topology)
	require.True(t, topology.ShouldConnect("A", "B"))
	require.True(t, topology.ShouldConnect("C", "B"))
	require.True(t, topology.ShouldConnect("C", "D"))
	require.False(t, topology.ShouldConnect("A", "C"))
}

func TestLineTopologyFactory_String_ReturnsExpectedFormat(t *testing.T) {
	factory := LineTopologyFactory{}
	require.Equal(t, "line", factory.String())
}

func TestRingTopologyFactory_Create_CreatesTopology(t *testing.T) {
	factory := RingTopologyFactory{}
	peers := map[PeerId]int{"A": 0, "B": 0, "C": 0, "D": 0}

	topology := factory.Create(peers)

	require.NotNil(t, topology)
	require.IsType(t, &RingTopology{}, topology)
	require.True(t, topology.ShouldConnect("A", "B"))
	require.True(t, topology.ShouldConnect("D", "A"))
	require.False(t, topology.ShouldConnect("A", "C"))
	require.False(t, topology.ShouldConnect("B", "D"))
}

func TestRingTopologyFactory_String_ReturnsExpectedFormat(t *testing.T) {
	factory := RingTopologyFactory{}
	require.Equal(t, "ring", factory.String())
}

func TestStarTopologyFactory_Create_CreatesTopology(t *testing.T) {
	factory := StarTopologyFactory{}
	peers := map[PeerId]int{"hub": 0, "spoke-A": 0, "spoke-B": 0, "spoke-C": 0}

	topology := factory.Create(peers)

	require.NotNil(t, topology)
	require.IsType(t, &StarTopology{}, topology)
	require.True(t, topology.ShouldConnect("hub", "spoke-A"))
	require.True(t, topology.ShouldConnect("spoke-B", "hub"))
	require.False(t, topology.ShouldConnect("spoke-A", "spoke-B"))
}

func TestStarTopologyFactory_Create_PanicsWithEmptyPeerList(t *testing.T) {
	factory := StarTopologyFactory{}
	require.Panics(t, func() {
		factory.Create(make(map[PeerId]int))
	})
}

func TestStarTopologyFactory_String_ReturnsExpectedFormat(t *testing.T) {
	factory := StarTopologyFactory{}
	require.Equal(t, "star", factory.String())
}

func TestRandomNaryGraphTopologyFactory_Create_CreatesTopology(t *testing.T) {
	factory := RandomNaryGraphTopologyFactory{N: 3, Seed: 42}
	peers := map[PeerId]int{"A": 0, "B": 0, "C": 0, "D": 0, "E": 0}

	topology := factory.Create(peers)

	require.NotNil(t, topology)
	require.IsType(t, &RandomNaryGraphTopology{}, topology)

	for peer := range peers {
		connectionCount := 0
		for otherPeer := range peers {
			if topology.ShouldConnect(peer, otherPeer) {
				connectionCount++
			}
		}
		require.LessOrEqual(t, connectionCount, 3)
	}
}

func TestRandomNaryGraphTopologyFactory_String_ReturnsExpectedFormat(t *testing.T) {
	factory := RandomNaryGraphTopologyFactory{N: 5, Seed: 123}
	require.Equal(t, "random-5-seed123", factory.String())
}

func TestTwoLayerTopologyFactory_Create_CreatesTopology(t *testing.T) {
	factory := TwoLayerTopologyFactory{Seed: 42}
	peers := map[PeerId]int{
		"v1":  TwoLayerValidatorLayer,
		"v2":  TwoLayerValidatorLayer,
		"nv1": TwoLayerNonValidatorLayer,
		"nv2": TwoLayerNonValidatorLayer,
	}

	topology := factory.Create(peers)

	require.NotNil(t, topology)
	require.IsType(t, &TwoLayerTopology{}, topology)

	// Validators should be fully meshed
	require.True(t, topology.ShouldConnect("v1", "v2"))
	require.True(t, topology.ShouldConnect("v2", "v1"))

	// Each non-validator should connect to at least one validator
	nv1ConnectsToValidator := topology.ShouldConnect("nv1", "v1") ||
		topology.ShouldConnect("nv1", "v2")
	require.True(t, nv1ConnectsToValidator)

	nv2ConnectsToValidator := topology.ShouldConnect("nv2", "v1") ||
		topology.ShouldConnect("nv2", "v2")
	require.True(t, nv2ConnectsToValidator)

	// Non-validators should not connect to each other
	require.False(t, topology.ShouldConnect("nv1", "nv2"))
}

func TestTwoLayerTopologyFactory_String_ReturnsExpectedFormat(t *testing.T) {
	factory := TwoLayerTopologyFactory{Seed: 123}
	require.Equal(t, "two-layer-seed123", factory.String())
}
