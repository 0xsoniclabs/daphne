package sim

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/central"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/autocracy"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/lachesis"
	"github.com/0xsoniclabs/daphne/daphne/consensus/streamlet"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/sim/scenario"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/mock/gomock"
)

func TestRun_SmokeTest(t *testing.T) {
	output := filepath.Join(t.TempDir(), "output.parquet")
	command := getRunCommand()
	require.NotNil(t, command)
	require.NoError(t, command.Run(t.Context(), []string{
		"run", "-s",
		"-o", output,
	}))
	require.FileExists(t, output)
}

func TestRun_InvalidOutputLocation_ReportsOutputError(t *testing.T) {
	command := getRunCommand()
	require.NotNil(t, command)
	err := command.Run(t.Context(), []string{
		"run", "-d", "100ms",
		"-o", t.TempDir(), // < can not write to a directory
	})
	require.ErrorContains(t, err, "is a directory")
}

func TestRun_InvalidNumberOfNodes_ReportsError(t *testing.T) {
	command := getRunCommand()
	require.NotNil(t, command)
	err := command.Run(t.Context(), []string{
		"run", "-n", "0",
		"-o", t.TempDir(), // < can not write to a directory
	})
	require.ErrorContains(t, err, "number of nodes must be positive")
}

func TestRunScenario_ForwardsErrorIfScenarioFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	scenario := scenario.NewMockScenario(ctrl)

	config := RunConfig{
		outputFile: filepath.Join(t.TempDir(), "output.parquet"),
	}

	issue := fmt.Errorf("scenario failed")
	scenario.EXPECT().Run(gomock.Any(), gomock.Any()).Return(issue)
	require.ErrorIs(t, runScenario(config, scenario), issue)
}

func TestGetBroadcastFactories_MapsProtocolNameToImplementation(t *testing.T) {
	tests := map[string]broadcast.Factory[types.Hash, types.Transaction]{
		"":           broadcast.NewDefault[types.Hash, types.Transaction],
		"default":    broadcast.NewDefault[types.Hash, types.Transaction],
		"bla":        broadcast.NewDefault[types.Hash, types.Transaction],
		"gossip":     broadcast.NewGossip[types.Hash, types.Transaction],
		"g":          broadcast.NewGossip[types.Hash, types.Transaction],
		"forwarding": broadcast.NewForwarding[types.Hash, types.Transaction],
		"f":          broadcast.NewForwarding[types.Hash, types.Transaction],
	}

	for name, expectedFactory := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			server := p2p.NewMockServer(ctrl)
			server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()

			factories := getBroadcastFactories(name)
			factory := broadcast.GetFactory[types.Hash, types.Transaction](factories)
			require.IsType(t, expectedFactory(server, nil), factory(server, nil))
		})
	}
}

func TestGetConsensusFactory_MapsProtocolNameToImplementation(t *testing.T) {
	tests := map[string]consensus.Factory{
		"c":         central.Factory{},
		"central":   central.Factory{},
		"s":         streamlet.Factory{},
		"streamlet": streamlet.Factory{},
		"a":         dag.Factory{LayeringFactory: autocracy.Factory{}},
		"autocrat":  dag.Factory{LayeringFactory: autocracy.Factory{}},
		"l":         dag.Factory{LayeringFactory: lachesis.Factory{}},
		"lachesis":  dag.Factory{LayeringFactory: lachesis.Factory{}},
		"":          central.Factory{},
		"bla":       central.Factory{},
	}

	for name, expectedFactory := range tests {
		t.Run(name, func(t *testing.T) {
			committee := consensus.Committee{}
			broadcastFactories := broadcast.Factories{}
			factory := getConsensusFactory(name, committee, broadcastFactories)
			require.IsType(t, expectedFactory, factory)
		})
	}
}

func TestGetNetworkTopology_MapsTopologyNameToImplementation(t *testing.T) {
	tests := map[string]struct {
		topologyFlag string
		numNodes     int
		expectedType p2p.NetworkTopology
		topologyN    int
		topologySeed int64
	}{
		"fully-meshed": {
			topologyFlag: "fully-meshed",
			numNodes:     5,
			expectedType: p2p.NewFullyMeshedTopology(),
		},
		"fully-meshed-short": {
			topologyFlag: "f",
			numNodes:     5,
			expectedType: p2p.NewFullyMeshedTopology(),
		},
		"line": {
			topologyFlag: "line",
			numNodes:     5,
			expectedType: p2p.NewLineTopology(makePeerIds(5)),
		},
		"line-short": {
			topologyFlag: "l",
			numNodes:     5,
			expectedType: p2p.NewLineTopology(makePeerIds(5)),
		},
		"ring": {
			topologyFlag: "ring",
			numNodes:     5,
			expectedType: p2p.NewRingTopology(makePeerIds(5)),
		},
		"ring-short": {
			topologyFlag: "r",
			numNodes:     5,
			expectedType: p2p.NewRingTopology(makePeerIds(5)),
		},
		"star": {
			topologyFlag: "star",
			numNodes:     5,
			expectedType: p2p.NewStarTopology(p2p.PeerId("N-001"), makePeerIds(5)),
		},
		"star-short": {
			topologyFlag: "s",
			numNodes:     5,
			expectedType: p2p.NewStarTopology(p2p.PeerId("N-001"), makePeerIds(5)),
		},
		"random": {
			topologyFlag: "random",
			numNodes:     5,
			topologyN:    3,
			topologySeed: 42,
			expectedType: p2p.NewRandomNaryGraphTopology(makePeerIds(5), 3, 42),
		},
		"random-short": {
			topologyFlag: "rand",
			numNodes:     5,
			topologyN:    2,
			topologySeed: 123,
			expectedType: p2p.NewRandomNaryGraphTopology(makePeerIds(5), 2, 123),
		},
		"unknown-defaults-to-fully-meshed": {
			topologyFlag: "unknown",
			numNodes:     5,
			expectedType: p2p.NewFullyMeshedTopology(),
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			// Create a mock command with flags.
			cmd := &cli.Command{}
			cmd.Flags = []cli.Flag{
				topologyFlag,
				topologyNFlag,
				topologySeedFlag,
			}

			// Build args based on test case.
			args := []string{"test", "--topology", testCase.topologyFlag}
			if testCase.topologyN > 0 {
				args = append(
					args,
					"--topology-n",
					fmt.Sprintf("%d", testCase.topologyN),
				)
			}
			if testCase.topologySeed > 0 {
				args = append(
					args,
					"--topology-seed",
					fmt.Sprintf("%d", testCase.topologySeed),
				)
			}

			require.NoError(t, cmd.Run(t.Context(), args))

			topology := getNetworkTopology(cmd, testCase.numNodes)
			require.IsType(t, testCase.expectedType, topology)

			// Verify topology behavior matches expected by testing all peer
			// connections.
			testPeers := makePeerIds(testCase.numNodes)
			for i := range testPeers {
				for j := range testPeers {
					if i != j {
						expected := testCase.expectedType.ShouldConnect(
							testPeers[i],
							testPeers[j],
						)
						actual := topology.ShouldConnect(testPeers[i], testPeers[j])
						require.Equal(
							t,
							expected,
							actual,
							"Topology connection behavior should match for %s -> %s",
							testPeers[i],
							testPeers[j],
						)
					}
				}
			}
		})
	}
}

func TestLoadScenario_PassesTopologyToScenario(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		numNodesFlag,
		txPerSecondFlag,
		durationFlag,
		broadcastProtocolFlag,
		topologyFlag,
		topologyNFlag,
		topologySeedFlag,
	}

	args := []string{"test", "--num-nodes", "5", "--topology", "ring"}
	require.NoError(t, cmd.Run(t.Context(), args))

	s, err := loadScenario(cmd)
	require.NoError(t, err)
	scenario := s.(*scenario.DemoScenario)
	require.NotNil(t, scenario)
	require.NotNil(t, scenario.Topology)

	// Verify it's a ring topology by checking expected connections.
	peerIds := makePeerIds(5)
	require.True(t, scenario.Topology.ShouldConnect(peerIds[0], peerIds[1]))
	require.True(t, scenario.Topology.ShouldConnect(peerIds[1], peerIds[2]))
	require.True(t, scenario.Topology.ShouldConnect(peerIds[2], peerIds[3]))
	require.True(t, scenario.Topology.ShouldConnect(peerIds[3], peerIds[4]))
	require.True(t, scenario.Topology.ShouldConnect(peerIds[4], peerIds[0]))
	require.False(t, scenario.Topology.ShouldConnect(peerIds[0], peerIds[2]))
	require.False(t, scenario.Topology.ShouldConnect(peerIds[0], peerIds[3]))
	require.False(t, scenario.Topology.ShouldConnect(peerIds[1], peerIds[3]))
	require.False(t, scenario.Topology.ShouldConnect(peerIds[1], peerIds[4]))
	require.False(t, scenario.Topology.ShouldConnect(peerIds[2], peerIds[4]))
}

// Helper function to generate peer IDs matching the DemoScenario convention.
func makePeerIds(numNodes int) []p2p.PeerId {
	peerIds := make([]p2p.PeerId, numNodes)
	for i := range numNodes {
		peerIds[i] = p2p.PeerId(fmt.Sprintf("N-%03d", i+1))
	}
	return peerIds
}
