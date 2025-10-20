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
