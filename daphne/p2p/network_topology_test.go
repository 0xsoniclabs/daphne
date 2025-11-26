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

func TestFullyMeshedTopologyFactory_Create_CreatesTopology(t *testing.T) {
	factory := FullyMeshedTopologyFactory{}
	peers := []PeerId{"A", "B", "C"}

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
	peers := []PeerId{"A", "B", "C", "D"}

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
	peers := []PeerId{"A", "B", "C", "D"}

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
	peers := []PeerId{"hub", "spoke-A", "spoke-B", "spoke-C"}

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
		factory.Create([]PeerId{})
	})
}

func TestStarTopologyFactory_String_ReturnsExpectedFormat(t *testing.T) {
	factory := StarTopologyFactory{}
	require.Equal(t, "star", factory.String())
}

func TestRandomNaryGraphTopologyFactory_Create_CreatesTopology(t *testing.T) {
	factory := RandomNaryGraphTopologyFactory{N: 3, Seed: 42}
	peers := []PeerId{"A", "B", "C", "D", "E"}

	topology := factory.Create(peers)

	require.NotNil(t, topology)
	require.IsType(t, &RandomNaryGraphTopology{}, topology)

	for _, peer := range peers {
		connectionCount := 0
		for _, otherPeer := range peers {
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
