// Copyright 2026 Sonic Operations Ltd
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

package sim

import (
	"fmt"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/central"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/autocracy"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/moira"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
	"github.com/0xsoniclabs/daphne/daphne/consensus/streamlet"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/sim/scenario"
	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/mock/gomock"
)

func TestEval_SmokeTest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "output.parquet")
		command := getEvalCommand()
		require.NotNil(t, command)
		require.NoError(t, command.Run(t.Context(), []string{
			"run", "-rt",
			"-o", output,
		}))
		require.FileExists(t, output)
	})
}

func TestEval_SimulatedTimeWorks(t *testing.T) {
	output := filepath.Join(t.TempDir(), "output.parquet")
	command := getEvalCommand()
	require.NotNil(t, command)
	require.NoError(t, command.Run(t.Context(), []string{
		"run",
		"-o", output,
	}))
	require.FileExists(t, output)
}

func TestEval_InvalidOutputLocation_ReportsOutputError(t *testing.T) {
	command := getEvalCommand()
	require.NotNil(t, command)
	err := command.Run(t.Context(), []string{
		"run", "-d", "100ms",
		"-o", t.TempDir(), // < can not write to a directory
	})
	require.ErrorContains(t, err, "is a directory")
}

func TestEval_InvalidNumberOfValidators_ReportsError(t *testing.T) {
	command := getEvalCommand()
	require.NotNil(t, command)
	err := command.Run(t.Context(), []string{
		"run", "-n", "0",
	})
	require.ErrorContains(t, err, "number of validators must be positive")
}

func TestEval_InvalidNumberOfRpcNodes_ReportsError(t *testing.T) {
	command := getEvalCommand()
	require.NotNil(t, command)
	err := command.Run(t.Context(), []string{
		"run", "--num-rpc-nodes", "0",
	})
	require.ErrorContains(t, err, "number of RPC nodes must be positive")
}

func TestEval_InvalidNumberOfObservers_ReportsError(t *testing.T) {
	command := getEvalCommand()
	require.NotNil(t, command)
	err := command.Run(t.Context(), []string{
		"run", "--num-observers", "-1",
	})
	require.ErrorContains(t, err, "number of observers cannot be negative")
}
func TestEvalScenario_ForwardsErrorIfScenarioFails(t *testing.T) {
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

			protocol := getBroadcastProtocol(name)
			factory := broadcast.GetFactory[types.Hash, types.Transaction](protocol)
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
		"a":         dag.Factory[payload.Transactions]{LayeringFactory: autocracy.Factory{}},
		"autocrat":  dag.Factory[payload.Transactions]{LayeringFactory: autocracy.Factory{}},
		"l":         dag.Factory[payload.Transactions]{LayeringFactory: moira.LachesisFactory{}},
		"lachesis":  dag.Factory[payload.Transactions]{LayeringFactory: moira.LachesisFactory{}},
		"":          central.Factory{},
		"bla":       central.Factory{},
	}

	for name, expectedFactory := range tests {
		t.Run(name, func(t *testing.T) {
			broadcastProtocol := broadcast.ProtocolGossip
			factory := getConsensusFactory(name, broadcastProtocol)
			require.IsType(t, expectedFactory, factory)
		})
	}
}

func TestGetNetworkTopology_MapsTopologyNameToImplementation(t *testing.T) {
	tests := map[string]struct {
		topologyFlag string
		numNodes     int
		expectedType p2p.TopologyFactory
		topologyN    int
		topologySeed int64
	}{
		"fully-meshed": {
			topologyFlag: "fully-meshed",
			numNodes:     5,
			expectedType: p2p.FullyMeshedTopologyFactory{},
		},
		"fully-meshed-short": {
			topologyFlag: "f",
			numNodes:     5,
			expectedType: p2p.FullyMeshedTopologyFactory{},
		},
		"line": {
			topologyFlag: "line",
			numNodes:     5,
			expectedType: p2p.LineTopologyFactory{},
		},
		"line-short": {
			topologyFlag: "l",
			numNodes:     5,
			expectedType: p2p.LineTopologyFactory{},
		},
		"ring": {
			topologyFlag: "ring",
			numNodes:     5,
			expectedType: p2p.RingTopologyFactory{},
		},
		"ring-short": {
			topologyFlag: "r",
			numNodes:     5,
			expectedType: p2p.RingTopologyFactory{},
		},
		"star": {
			topologyFlag: "star",
			numNodes:     5,
			expectedType: p2p.StarTopologyFactory{},
		},
		"star-short": {
			topologyFlag: "s",
			numNodes:     5,
			expectedType: p2p.StarTopologyFactory{},
		},
		"random": {
			topologyFlag: "random",
			numNodes:     5,
			topologyN:    3,
			topologySeed: 42,
			expectedType: p2p.RandomNaryGraphTopologyFactory{N: 3, Seed: 42},
		},
		"random-short": {
			topologyFlag: "rand",
			numNodes:     5,
			topologyN:    2,
			topologySeed: 123,
			expectedType: p2p.RandomNaryGraphTopologyFactory{N: 2, Seed: 123},
		},
		"unknown-defaults-to-fully-meshed": {
			topologyFlag: "unknown",
			numNodes:     5,
			expectedType: p2p.FullyMeshedTopologyFactory{},
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

			factory := getNetworkTopology(cmd)
			require.IsType(t, testCase.expectedType, factory)

			// Verify topology behavior matches expected by creating topologies
			// from both factories and testing all peer connections.
			testPeers := makePeerIds(testCase.numNodes)
			// Convert peer slice to map with all peers in layer 0
			testPeerMap := make(map[p2p.PeerId]int, len(testPeers))
			for _, peerId := range testPeers {
				testPeerMap[peerId] = 0
			}
			expectedTopology := testCase.expectedType.Create(testPeerMap)
			actualTopology := factory.Create(testPeerMap)
			for i := range testPeers {
				for j := range testPeers {
					if i != j {
						expected := expectedTopology.ShouldConnect(
							testPeers[i],
							testPeers[j],
						)
						actual := actualTopology.ShouldConnect(testPeers[i], testPeers[j])
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
		numValidatorsFlag,
		numRpcNodesFlag,
		txPerSecondFlag,
		durationFlag,
		broadcastProtocolFlag,
		topologyFlag,
		topologyNFlag,
		topologySeedFlag,
	}

	args := []string{"test", "--num-validators", "4", "--topology", "ring"}
	require.NoError(t, cmd.Run(t.Context(), args))

	s, err := loadScenario(cmd)
	require.NoError(t, err)
	scenario := s.(*scenario.DemoScenario)
	require.NotNil(t, scenario)
	require.NotNil(t, scenario.Topology)

	// Verify it's a ring topology by checking expected connections.
	peerIds := makePeerIds(5)
	peerMap := make(map[p2p.PeerId]int, len(peerIds))
	for _, peerId := range peerIds {
		peerMap[peerId] = 0
	}
	topology := scenario.Topology.Create(peerMap)
	require.True(t, topology.ShouldConnect(peerIds[0], peerIds[1]))
	require.True(t, topology.ShouldConnect(peerIds[1], peerIds[2]))
	require.True(t, topology.ShouldConnect(peerIds[2], peerIds[3]))
	require.True(t, topology.ShouldConnect(peerIds[3], peerIds[4]))
	require.True(t, topology.ShouldConnect(peerIds[4], peerIds[0]))
	require.False(t, topology.ShouldConnect(peerIds[0], peerIds[2]))
	require.False(t, topology.ShouldConnect(peerIds[0], peerIds[3]))
	require.False(t, topology.ShouldConnect(peerIds[1], peerIds[3]))
	require.False(t, topology.ShouldConnect(peerIds[1], peerIds[4]))
	require.False(t, topology.ShouldConnect(peerIds[2], peerIds[4]))
}

// Helper function to generate peer IDs matching the DemoScenario convention.
func makePeerIds(numNodes int) []p2p.PeerId {
	peerIds := make([]p2p.PeerId, numNodes)
	for i := range numNodes {
		peerIds[i] = p2p.PeerId(fmt.Sprintf("N-%03d", i+1))
	}
	return peerIds
}

func TestGetNetworkGeography_None_ReturnsZeroLatencyModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{networkLatencyModelFlag}

	args := []string{"test", "--network-latency-model", "none"}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getNetworkGeography(cmd)
	require.NoError(t, err)

	require.Zero(t, model.GetLocalSendLatency().SampleDuration())
	require.Zero(t, model.GetLocalDeliveryLatency().SampleDuration())
}

func TestGetNetworkGeography_Empty_ReturnsZeroLatencyModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{networkLatencyModelFlag}

	args := []string{"test"}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getNetworkGeography(cmd)
	require.NoError(t, err)

	require.Zero(t, model.GetLocalSendLatency().SampleDuration())
	require.Zero(t, model.GetLocalDeliveryLatency().SampleDuration())
}

func TestGetNetworkGeography_Fixed_ReturnsFixedDelayModelAndAppliesDelays(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		networkLatencyModelFlag,
		networkLatencyFixedSendFlag,
		networkLatencyFixedDeliveryFlag,
	}

	args := []string{
		"test",
		"--network-latency-model", "fixed",
		"--network-latency-fixed-send", "10ms",
		"--network-latency-fixed-delivery", "20ms",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getNetworkGeography(cmd)
	require.NoError(t, err)
	require.NotNil(t, model)

	sendDelay := model.GetLocalSendLatency().SampleDuration()
	require.Equal(t, 10*time.Millisecond, sendDelay)

	deliveryDelay := model.GetLocalDeliveryLatency().SampleDuration()
	require.Equal(t, 20*time.Millisecond, deliveryDelay)
}

func TestGetNetworkGeography_SampledPresetFast_ReturnsSampledDelayModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		networkLatencyModelFlag,
		networkLatencyPresetFlag,
	}

	args := []string{
		"test",
		"--network-latency-model", "sampled",
		"--network-latency-preset", "fast",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getNetworkGeography(cmd)
	require.NoError(t, err)
	require.NotNil(t, model)

	// Verify quantiles by sampling - Fast preset:
	// P10=5ms, P90=15ms for send;
	// P10=2ms, P90=8ms for delivery

	verifyNetworkDelayQuantiles(t, model, "send", 0.1, 5*time.Millisecond, 0.9, 15*time.Millisecond)
	verifyNetworkDelayQuantiles(t, model, "delivery", 0.1, 2*time.Millisecond, 0.9, 8*time.Millisecond)
}

func TestGetNetworkGeography_SampledPresetSlow_ReturnsSampledDelayModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		networkLatencyModelFlag,
		networkLatencyPresetFlag,
	}

	args := []string{
		"test",
		"--network-latency-model", "sampled",
		"--network-latency-preset", "slow",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getNetworkGeography(cmd)
	require.NoError(t, err)
	require.NotNil(t, model)

	// Verify quantiles by sampling - Slow preset:
	// P10=50ms, P90=150ms for send; P10=30ms, P90=100ms for delivery
	verifyNetworkDelayQuantiles(t, model, "send", 0.1, 50*time.Millisecond, 0.9, 150*time.Millisecond)
	verifyNetworkDelayQuantiles(t, model, "delivery", 0.1, 30*time.Millisecond, 0.9, 100*time.Millisecond)
}

func TestGetNetworkGeography_SampledPresetNoisy_ReturnsSampledDelayModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		networkLatencyModelFlag,
		networkLatencyPresetFlag,
	}

	args := []string{
		"test",
		"--network-latency-model", "sampled",
		"--network-latency-preset", "noisy",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getNetworkGeography(cmd)
	require.NoError(t, err)
	require.NotNil(t, model)

	// Verify quantiles by sampling - Noisy preset:
	// P10=10ms, P95=500ms for send; P10=5ms, P95=300ms for delivery
	verifyNetworkDelayQuantiles(t, model, "send", 0.1, 10*time.Millisecond, 0.95, 500*time.Millisecond)
	verifyNetworkDelayQuantiles(t, model, "delivery", 0.1, 5*time.Millisecond, 0.95, 300*time.Millisecond)
}

func TestGetNetworkGeography_SampledTwoPercentiles_ReturnsSampledDelayModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		networkLatencyModelFlag,
		networkLatencySendP1Flag,
		networkLatencySendP1ValueFlag,
		networkLatencySendP2Flag,
		networkLatencySendP2ValueFlag,
		networkLatencyDeliveryP1Flag,
		networkLatencyDeliveryP1ValueFlag,
		networkLatencyDeliveryP2Flag,
		networkLatencyDeliveryP2ValueFlag,
	}

	args := []string{
		"test",
		"--network-latency-model", "sampled",
		"--network-latency-send-p1", "0.1",
		"--network-latency-send-p1-value", "20ms",
		"--network-latency-send-p2", "0.95",
		"--network-latency-send-p2-value", "200ms",
		"--network-latency-delivery-p1", "0.1",
		"--network-latency-delivery-p1-value", "10ms",
		"--network-latency-delivery-p2", "0.90",
		"--network-latency-delivery-p2-value", "100ms",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getNetworkGeography(cmd)
	require.NoError(t, err)
	require.NotNil(t, model)
}

func TestGetNetworkGeography_UnknownModel_ReturnsError(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{networkLatencyModelFlag}

	args := []string{"test", "--network-latency-model", "unknown"}
	require.NoError(t, cmd.Run(t.Context(), args))

	_, err := getNetworkGeography(cmd)
	require.ErrorContains(t, err, "unknown network latency model")
}

func TestGetNetworkGeography_SampledMissingParameters_ReturnsError(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		networkLatencyModelFlag,
		networkLatencySendP1Flag,
		networkLatencySendP1ValueFlag,
		networkLatencySendP2Flag,
		networkLatencySendP2ValueFlag,
		networkLatencyDeliveryP1Flag,
	}

	args := []string{
		"test",
		"--network-latency-model", "sampled",
		"--network-latency-send-p1", "0.1",
		"--network-latency-send-p1-value", "20ms",
		"--network-latency-send-p2", "0.95",
		"--network-latency-send-p2-value", "200ms",
		"--network-latency-delivery-p1", "0.1",
		// Missing delivery p1-value, p2, and p2-value
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	_, err := getNetworkGeography(cmd)
	require.ErrorContains(t, err, "failed to configure delivery latency distribution")
}

func TestLoadScenario_PassesLatencyModelToScenario(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		numValidatorsFlag,
		numRpcNodesFlag,
		txPerSecondFlag,
		durationFlag,
		broadcastProtocolFlag,
		consensusProtocolFlag,
		topologyFlag,
		topologyNFlag,
		topologySeedFlag,
		networkLatencyModelFlag,
		networkLatencyFixedSendFlag,
		networkLatencyFixedDeliveryFlag,
	}

	args := []string{
		"test",
		"--num-validators", "3",
		"--network-latency-model", "fixed",
		"--network-latency-fixed-send", "10ms",
		"--network-latency-fixed-delivery", "20ms",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	s, err := loadScenario(cmd)
	require.NoError(t, err)
	scenario := s.(*scenario.DemoScenario)
	require.NotNil(t, scenario)
	require.NotNil(t, scenario.NetworkGeography)
}

func TestLoadScenario_NoLatencyModel_ScenarioHasZeroLatencyModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		numValidatorsFlag,
		numRpcNodesFlag,
		txPerSecondFlag,
		durationFlag,
		broadcastProtocolFlag,
		consensusProtocolFlag,
		topologyFlag,
		topologyNFlag,
		topologySeedFlag,
		networkLatencyModelFlag,
	}

	args := []string{"test", "--num-validators", "3"}
	require.NoError(t, cmd.Run(t.Context(), args))

	s, err := loadScenario(cmd)
	require.NoError(t, err)

	scenario := s.(*scenario.DemoScenario)
	require.NotNil(t, scenario)
	require.Zero(t, scenario.NetworkGeography.GetLocalSendLatency().SampleDuration())
	require.Zero(t, scenario.NetworkGeography.GetLocalDeliveryLatency().SampleDuration())
}

func TestLoadScenario_InvalidLatencyModel_ReturnsError(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		numValidatorsFlag,
		numRpcNodesFlag,
		txPerSecondFlag,
		durationFlag,
		broadcastProtocolFlag,
		consensusProtocolFlag,
		topologyFlag,
		topologyNFlag,
		topologySeedFlag,
		networkLatencyModelFlag,
	}

	args := []string{"test", "--num-validators", "3", "--network-latency-model", "invalid"}
	require.NoError(t, cmd.Run(t.Context(), args))

	_, err := loadScenario(cmd)
	require.ErrorContains(t, err, "failed to configure network latency model")
}

func TestGetNetworkGeography_SampledInvalidSendLatency_ReturnsError(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		networkLatencyModelFlag,
		networkLatencySendP1Flag,
		networkLatencySendP1ValueFlag,
		networkLatencySendP2Flag,
		networkLatencySendP2ValueFlag,
		networkLatencyDeliveryP1Flag,
		networkLatencyDeliveryP1ValueFlag,
		networkLatencyDeliveryP2Flag,
		networkLatencyDeliveryP2ValueFlag,
	}

	// Provide invalid send latency (percentile > 1.0)
	args := []string{
		"test",
		"--network-latency-model", "sampled",
		"--network-latency-send-p1", "0.1",
		"--network-latency-send-p1-value", "50ms",
		"--network-latency-send-p2", "1.5",
		"--network-latency-send-p2-value", "200ms",
		"--network-latency-delivery-p1", "0.1",
		"--network-latency-delivery-p1-value", "10ms",
		"--network-latency-delivery-p2", "0.9",
		"--network-latency-delivery-p2-value", "100ms",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	_, err := getNetworkGeography(cmd)
	require.ErrorContains(t, err, "failed to configure send latency distribution")
}

func TestGetNetworkGeography_SampledTwoPercentilesInconsistent_ReturnsError(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		networkLatencyModelFlag,
		networkLatencySendP1Flag,
		networkLatencySendP1ValueFlag,
		networkLatencySendP2Flag,
		networkLatencySendP2ValueFlag,
		networkLatencyDeliveryP1Flag,
		networkLatencyDeliveryP1ValueFlag,
		networkLatencyDeliveryP2Flag,
		networkLatencyDeliveryP2ValueFlag,
	}

	// Invalid: p1Target == p2Target (must be different)
	args := []string{
		"test",
		"--network-latency-model", "sampled",
		"--network-latency-send-p1", "0.1",
		"--network-latency-send-p1-value", "50ms",
		"--network-latency-send-p2", "0.9",
		"--network-latency-send-p2-value", "50ms",
		"--network-latency-delivery-p1", "0.1",
		"--network-latency-delivery-p1-value", "10ms",
		"--network-latency-delivery-p2", "0.9",
		"--network-latency-delivery-p2-value", "100ms",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	_, err := getNetworkGeography(cmd)
	require.ErrorContains(t, err, "send two-percentile configuration")
}

func TestGetStateDelayModel_None_ReturnsNil(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{stateDelayModelFlag}

	args := []string{"test", "--state-delay-model", "none"}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getStateDelayModel(cmd)
	require.NoError(t, err)
	require.Nil(t, model)
}

func TestGetStateDelayModel_Empty_ReturnsDefaultModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{stateDelayModelFlag}

	args := []string{"test"}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getStateDelayModel(cmd)
	require.NoError(t, err)
	require.Equal(t, getDefaultStateProcessingLatencyModel(), model)
}

func TestGetStateDelayModel_Fitted_ReturnsNonNilDefault(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{stateDelayModelFlag}

	args := []string{"test"}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getStateDelayModel(cmd)
	require.NoError(t, err)
	require.Equal(t, getDefaultStateProcessingLatencyModel(), model)
}

func TestGetStateDelayModel_Fixed_ReturnsFixedDelayModelAndAppliesDelays(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		stateDelayModelFlag,
		stateDelayFixedTransactionFlag,
		stateDelayFixedFinalizationFlag,
	}

	args := []string{
		"test",
		"--state-delay-model", "fixed",
		"--state-delay-fixed-transaction", "5ms",
		"--state-delay-fixed-finalization", "10ms",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getStateDelayModel(cmd)
	require.NoError(t, err)
	require.NotNil(t, model)
	require.IsType(t, &state.FixedProcessingDelayModel{}, model)

	tx := types.Transaction{From: 0, To: 1, Nonce: 1}
	txDelay := model.GetTransactionDelay(tx)
	require.Equal(t, 5*time.Millisecond, txDelay)

	finalizationDelay := model.GetBlockFinalizationDelay(1, nil)
	require.Equal(t, 10*time.Millisecond, finalizationDelay)
}

func TestGetStateDelayModel_SampledPresetFast_ReturnsSampledDelayModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		stateDelayModelFlag,
		stateDelayPresetFlag,
	}

	args := []string{
		"test",
		"--state-delay-model", "sampled",
		"--state-delay-preset", "fast",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getStateDelayModel(cmd)
	require.NoError(t, err)
	require.NotNil(t, model)
	require.IsType(t, &state.SampledProcessingDelayModel{}, model)

	// Verify quantiles by sampling - Fast preset:
	// P10=1ms, P90=3ms for transaction;
	// P10=2ms, P90=6ms for finalization
	sampledModel := model.(*state.SampledProcessingDelayModel)

	verifyStateDelayQuantiles(t, sampledModel, "transaction", 0.1, 1*time.Millisecond, 0.9, 3*time.Millisecond)
	verifyStateDelayQuantiles(t, sampledModel, "finalization", 0.1, 2*time.Millisecond, 0.9, 6*time.Millisecond)
}

func TestGetStateDelayModel_SampledPresetSlow_ReturnsSampledDelayModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		stateDelayModelFlag,
		stateDelayPresetFlag,
	}

	args := []string{
		"test",
		"--state-delay-model", "sampled",
		"--state-delay-preset", "slow",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getStateDelayModel(cmd)
	require.NoError(t, err)
	require.NotNil(t, model)
	require.IsType(t, &state.SampledProcessingDelayModel{}, model)

	// Verify quantiles by sampling - Slow preset:
	// P10=5ms, P90=20ms for transaction; P10=10ms, P90=50ms for finalization
	sampledModel := model.(*state.SampledProcessingDelayModel)
	verifyStateDelayQuantiles(t, sampledModel, "transaction", 0.1, 5*time.Millisecond, 0.9, 20*time.Millisecond)
	verifyStateDelayQuantiles(t, sampledModel, "finalization", 0.1, 10*time.Millisecond, 0.9, 50*time.Millisecond)
}

func TestGetStateDelayModel_SampledPresetNoisy_ReturnsSampledDelayModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		stateDelayModelFlag,
		stateDelayPresetFlag,
	}

	args := []string{
		"test",
		"--state-delay-model", "sampled",
		"--state-delay-preset", "noisy",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getStateDelayModel(cmd)
	require.NoError(t, err)
	require.NotNil(t, model)
	require.IsType(t, &state.SampledProcessingDelayModel{}, model)

	// Verify quantiles by sampling - Noisy preset:
	// P10=1ms, P95=100ms for transaction; P10=2ms, P95=200ms for finalization
	sampledModel := model.(*state.SampledProcessingDelayModel)
	verifyStateDelayQuantiles(t, sampledModel, "transaction", 0.1, 1*time.Millisecond, 0.95, 100*time.Millisecond)
	verifyStateDelayQuantiles(t, sampledModel, "finalization", 0.1, 2*time.Millisecond, 0.95, 200*time.Millisecond)
}

func TestGetStateDelayModel_SampledTwoPercentiles_ReturnsSampledDelayModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		stateDelayModelFlag,
		stateDelayTransactionP1Flag,
		stateDelayTransactionP1ValueFlag,
		stateDelayTransactionP2Flag,
		stateDelayTransactionP2ValueFlag,
		stateDelayFinalizationP1Flag,
		stateDelayFinalizationP1ValueFlag,
		stateDelayFinalizationP2Flag,
		stateDelayFinalizationP2ValueFlag,
	}

	args := []string{
		"test",
		"--state-delay-model", "sampled",
		"--state-delay-transaction-p1", "0.1",
		"--state-delay-transaction-p1-value", "10ms",
		"--state-delay-transaction-p2", "0.95",
		"--state-delay-transaction-p2-value", "50ms",
		"--state-delay-finalization-p1", "0.1",
		"--state-delay-finalization-p1-value", "20ms",
		"--state-delay-finalization-p2", "0.90",
		"--state-delay-finalization-p2-value", "100ms",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	model, err := getStateDelayModel(cmd)
	require.NoError(t, err)
	require.NotNil(t, model)
	require.IsType(t, &state.SampledProcessingDelayModel{}, model)
}

func TestGetStateDelayModel_UnknownModel_ReturnsError(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{stateDelayModelFlag}

	args := []string{"test", "--state-delay-model", "unknown"}
	require.NoError(t, cmd.Run(t.Context(), args))

	_, err := getStateDelayModel(cmd)
	require.ErrorContains(t, err, "unknown state delay model")
}

func TestGetStateDelayModel_SampledMissingParameters_ReturnsError(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		stateDelayModelFlag,
		stateDelayTransactionP1Flag,
		stateDelayTransactionP1ValueFlag,
		stateDelayTransactionP2Flag,
		stateDelayTransactionP2ValueFlag,
		stateDelayFinalizationP1Flag,
	}

	args := []string{
		"test",
		"--state-delay-model", "sampled",
		"--state-delay-transaction-p1", "0.1",
		"--state-delay-transaction-p1-value", "10ms",
		"--state-delay-transaction-p2", "0.95",
		"--state-delay-transaction-p2-value", "50ms",
		"--state-delay-finalization-p1", "0.1",
		// Missing finalization p1-value, p2, and p2-value
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	_, err := getStateDelayModel(cmd)
	require.ErrorContains(t, err, "failed to configure finalization delay distribution")
}

func TestLoadScenario_PassesStateDelayModelToScenario(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		numValidatorsFlag,
		numRpcNodesFlag,
		txPerSecondFlag,
		durationFlag,
		broadcastProtocolFlag,
		consensusProtocolFlag,
		topologyFlag,
		topologyNFlag,
		topologySeedFlag,
		networkLatencyModelFlag,
		stateDelayModelFlag,
		stateDelayFixedTransactionFlag,
		stateDelayFixedFinalizationFlag,
	}

	args := []string{
		"test",
		"--num-validators", "3",
		"--state-delay-model", "fixed",
		"--state-delay-fixed-transaction", "5ms",
		"--state-delay-fixed-finalization", "10ms",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	s, err := loadScenario(cmd)
	require.NoError(t, err)
	scenario := s.(*scenario.DemoScenario)
	require.NotNil(t, scenario)
	require.NotNil(t, scenario.StateProcessingDelayModel)
	require.IsType(t, &state.FixedProcessingDelayModel{}, scenario.StateProcessingDelayModel)
}

func TestLoadScenario_NoStateDelayModel_ScenarioHasDefaultStateDelayModel(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		numValidatorsFlag,
		numRpcNodesFlag,
		txPerSecondFlag,
		durationFlag,
		broadcastProtocolFlag,
		consensusProtocolFlag,
		topologyFlag,
		topologyNFlag,
		topologySeedFlag,
		networkLatencyModelFlag,
		stateDelayModelFlag,
	}

	args := []string{"test", "--num-validators", "3"}
	require.NoError(t, cmd.Run(t.Context(), args))

	s, err := loadScenario(cmd)
	require.NoError(t, err)
	scenario := s.(*scenario.DemoScenario)
	require.NotNil(t, scenario)
	require.Equal(t,
		getDefaultStateProcessingLatencyModel(),
		scenario.StateProcessingDelayModel,
	)
}

func TestLoadScenario_InvalidStateDelayModel_ReturnsError(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		numValidatorsFlag,
		numRpcNodesFlag,
		txPerSecondFlag,
		durationFlag,
		broadcastProtocolFlag,
		consensusProtocolFlag,
		topologyFlag,
		topologyNFlag,
		topologySeedFlag,
		networkLatencyModelFlag,
		stateDelayModelFlag,
	}

	args := []string{"test", "--num-validators", "3", "--state-delay-model", "invalid"}
	require.NoError(t, cmd.Run(t.Context(), args))

	_, err := loadScenario(cmd)
	require.ErrorContains(t, err, "failed to configure state delay model")
}

func TestGetStateDelayModel_SampledInvalidPercentile_ReturnsError(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		stateDelayModelFlag,
		stateDelayTransactionP1Flag,
		stateDelayTransactionP1ValueFlag,
		stateDelayTransactionP2Flag,
		stateDelayTransactionP2ValueFlag,
		stateDelayFinalizationP1Flag,
		stateDelayFinalizationP1ValueFlag,
		stateDelayFinalizationP2Flag,
		stateDelayFinalizationP2ValueFlag,
	}

	// Provide invalid transaction delay (percentile > 1.0)
	args := []string{
		"test",
		"--state-delay-model", "sampled",
		"--state-delay-transaction-p1", "0.1",
		"--state-delay-transaction-p1-value", "10ms",
		"--state-delay-transaction-p2", "1.5",
		"--state-delay-transaction-p2-value", "50ms",
		"--state-delay-finalization-p1", "0.1",
		"--state-delay-finalization-p1-value", "20ms",
		"--state-delay-finalization-p2", "0.9",
		"--state-delay-finalization-p2-value", "100ms",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	_, err := getStateDelayModel(cmd)
	require.ErrorContains(t, err, "failed to configure transaction delay distribution")
}

func TestGetStateDelayModel_SampledTwoPercentilesInconsistent_ReturnsError(t *testing.T) {
	cmd := &cli.Command{}
	cmd.Flags = []cli.Flag{
		stateDelayModelFlag,
		stateDelayTransactionP1Flag,
		stateDelayTransactionP1ValueFlag,
		stateDelayTransactionP2Flag,
		stateDelayTransactionP2ValueFlag,
		stateDelayFinalizationP1Flag,
		stateDelayFinalizationP1ValueFlag,
		stateDelayFinalizationP2Flag,
		stateDelayFinalizationP2ValueFlag,
	}

	// Invalid: p1Target == p2Target (must be different)
	args := []string{
		"test",
		"--state-delay-model", "sampled",
		"--state-delay-transaction-p1", "0.1",
		"--state-delay-transaction-p1-value", "10ms",
		"--state-delay-transaction-p2", "0.9",
		"--state-delay-transaction-p2-value", "10ms",
		"--state-delay-finalization-p1", "0.1",
		"--state-delay-finalization-p1-value", "20ms",
		"--state-delay-finalization-p2", "0.9",
		"--state-delay-finalization-p2-value", "100ms",
	}
	require.NoError(t, cmd.Run(t.Context(), args))

	_, err := getStateDelayModel(cmd)
	require.ErrorContains(t, err, "transaction two-percentile configuration")
}

// verifyNetworkDelayQuantiles verifies that the distribution quantiles match the expected
// values within a tight tolerance.
func verifyNetworkDelayQuantiles(
	t *testing.T,
	model scenario.NetworkGeography,
	delayType string,
	p1 float64,
	p1Expected time.Duration,
	p2 float64,
	p2Expected time.Duration,
) {
	t.Helper()

	var dist utils.Distribution
	if delayType == "send" {
		dist = model.GetLocalSendLatency()
	} else {
		dist = model.GetLocalDeliveryLatency()
	}

	require.NotNil(t, dist, "%s distribution should not be nil", delayType)

	// Cast to LogNormalDistribution to access Dist field
	logNormalDist, ok := dist.(*utils.LogNormalDistribution)
	require.True(t, ok, "%s distribution should be *LogNormalDistribution", delayType)

	// Get quantiles directly from the distribution
	actualP1 := logNormalDist.Dist.Quantile(p1)
	actualP2 := logNormalDist.Dist.Quantile(p2)

	// Convert to time.Duration (quantiles are in units, need to scale)
	// The distribution stores values in the configured timeUnit
	actualP1Duration := time.Duration(actualP1 * float64(time.Nanosecond))
	actualP2Duration := time.Duration(actualP2 * float64(time.Nanosecond))

	// Verify quantiles match expected values within tight tolerance (1e-6 relative error)
	require.InDelta(t, float64(p1Expected), float64(actualP1Duration), 1e-6*float64(p1Expected),
		"%s latency P%.0f: expected %v, got %v", delayType, p1*100, p1Expected, actualP1Duration)
	require.InDelta(t, float64(p2Expected), float64(actualP2Duration), 1e-6*float64(p2Expected),
		"%s latency P%.0f: expected %v, got %v", delayType, p2*100, p2Expected, actualP2Duration)
}

// verifyStateDelayQuantiles verifies that the distribution quantiles match the
// expected values within a tight tolerance.
func verifyStateDelayQuantiles(
	t *testing.T,
	model *state.SampledProcessingDelayModel,
	delayType string,
	p1 float64,
	p1Expected time.Duration,
	p2 float64,
	p2Expected time.Duration,
) {
	t.Helper()

	var dist utils.Distribution
	if delayType == "transaction" {
		dist = model.GetBaseTransactionDistribution()
	} else {
		dist = model.GetBaseBlockFinalizationDistribution()
	}

	require.NotNil(t, dist, "%s distribution should not be nil", delayType)

	// Cast to LogNormalDistribution to access Dist field
	logNormalDist, ok := dist.(*utils.LogNormalDistribution)
	require.True(t, ok, "%s distribution should be *LogNormalDistribution", delayType)

	// Get quantiles directly from the distribution
	actualP1 := logNormalDist.Dist.Quantile(p1)
	actualP2 := logNormalDist.Dist.Quantile(p2)

	// Convert to time.Duration (quantiles are in units, need to scale)
	// The distribution stores values in the configured timeUnit
	actualP1Duration := time.Duration(actualP1 * float64(time.Nanosecond))
	actualP2Duration := time.Duration(actualP2 * float64(time.Nanosecond))

	// Verify quantiles match expected values within tight tolerance (1e-6 relative error)
	require.InDelta(t, float64(p1Expected), float64(actualP1Duration), 1e-6*float64(p1Expected),
		"%s delay P%.0f: expected %v, got %v", delayType, p1*100, p1Expected, actualP1Duration)
	require.InDelta(t, float64(p2Expected), float64(actualP2Duration), 1e-6*float64(p2Expected),
		"%s delay P%.0f: expected %v, got %v", delayType, p2*100, p2Expected, actualP2Duration)
}
