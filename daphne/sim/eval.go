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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/utils"
	"github.com/urfave/cli/v3"
)

var (
	outputFileFlag = &cli.StringFlag{
		Name:    "output-file",
		Aliases: []string{"o"},
		Usage:   "Path to the output file for the simulation results",
		Value:   "output.parquet",
	}
	durationFlag = &cli.DurationFlag{
		Name:    "duration",
		Aliases: []string{"d"},
		Usage:   "Duration of the simulation",
		Value:   5 * time.Second,
	}
	numValidatorsFlag = &cli.IntFlag{
		Name:    "num-validators",
		Aliases: []string{"n"},
		Usage:   "Number of validators in the simulation",
		Value:   3,
	}
	numRpcNodesFlag = &cli.IntFlag{
		Name:  "num-rpc-nodes",
		Usage: "Number of RPC nodes in the simulation",
		Value: 1,
	}
	numObserversFlag = &cli.IntFlag{
		Name:  "num-observers",
		Usage: "Number of observer nodes in the simulation",
		Value: 0,
	}
	txPerSecondFlag = &cli.IntFlag{
		Name:    "tx-per-second",
		Aliases: []string{"t"},
		Usage:   "Number of transactions per second",
		Value:   100,
	}
	realTimeFlag = &cli.BoolFlag{
		Name:    "real-time",
		Aliases: []string{"rt"},
		Usage:   "Run the simulation in real time mode, instead of sim time mode",
		Value:   false,
	}
	broadcastProtocolFlag = &cli.StringFlag{
		Name:    "broadcast-protocol",
		Aliases: []string{"b"},
		Usage:   "Broadcast protocol to use (gossip, forwarding, etc.)",
		Value:   "gossip",
	}
	consensusProtocolFlag = &cli.StringFlag{
		Name:    "consensus-protocol",
		Aliases: []string{"c"},
		Usage:   "Consensus protocol to use (central, streamlet, etc.)",
		Value:   "central",
	}
	topologyFlag = &cli.StringFlag{
		Name:    "topology",
		Aliases: []string{"T"},
		Usage:   "Network topology to use (fully-meshed, line, ring, star, random)",
		Value:   "fully-meshed",
	}
	topologyNFlag = &cli.IntFlag{
		Name:    "topology-n",
		Aliases: []string{"Tn"},
		Usage:   "Number of connections per peer for random topology",
		Value:   3,
	}
	topologySeedFlag = &cli.Int64Flag{
		Name:    "topology-seed",
		Aliases: []string{"Ts"},
		Usage:   "Random seed for random topology",
		Value:   0,
	}
	networkLatencyModelFlag = &cli.StringFlag{
		Name:    "network-latency-model",
		Aliases: []string{"Nlm"},
		Usage:   "Network latency model to use (none, fixed, sampled)",
		Value:   "none",
	}
	networkLatencyPresetFlag = &cli.StringFlag{
		Name:    "network-latency-preset",
		Aliases: []string{"Nlp"},
		Usage:   "Network latency preset to use (fast, slow, noisy). Overrides individual p1/p2 flags when used with sampled model.",
		Value:   "",
	}
	networkLatencyFixedSendFlag = &cli.DurationFlag{
		Name:    "network-latency-fixed-send",
		Aliases: []string{"Nfs"},
		Usage:   "Fixed send latency for network messages (only used with fixed latency model)",
		Value:   0,
	}
	networkLatencyFixedDeliveryFlag = &cli.DurationFlag{
		Name:    "network-latency-fixed-delivery",
		Aliases: []string{"Nfd"},
		Usage:   "Fixed delivery latency for network messages (only used with fixed latency model)",
		Value:   0,
	}
	// Sampled send latency - Two Percentiles
	networkLatencySendP1Flag = &cli.Float64Flag{
		Name:    "network-latency-send-p1",
		Aliases: []string{"Nsp1"},
		Usage:   "First percentile for send latency (e.g., 0.1 for P10). Use with --network-latency-send-p1-value, --network-latency-send-p2, --network-latency-send-p2-value",
		Value:   0,
	}
	networkLatencySendP1ValueFlag = &cli.DurationFlag{
		Name:    "network-latency-send-p1-value",
		Aliases: []string{"Nsp1v"},
		Usage:   "Target value for send latency p1. Use with --network-latency-send-p1, --network-latency-send-p2, --network-latency-send-p2-value",
		Value:   0,
	}
	networkLatencySendP2Flag = &cli.Float64Flag{
		Name:    "network-latency-send-p2",
		Aliases: []string{"Nsp2"},
		Usage:   "Second percentile for send latency (e.g., 0.95 for P95). Use with --network-latency-send-p1, --network-latency-send-p1-value, --network-latency-send-p2-value",
		Value:   0,
	}
	networkLatencySendP2ValueFlag = &cli.DurationFlag{
		Name:    "network-latency-send-p2-value",
		Aliases: []string{"Nsp2v"},
		Usage:   "Target value for send latency p2. Use with --network-latency-send-p1, --network-latency-send-p1-value, --network-latency-send-p2",
		Value:   0,
	}
	// Sampled delivery latency - Two Percentiles
	networkLatencyDeliveryP1Flag = &cli.Float64Flag{
		Name:    "network-latency-delivery-p1",
		Aliases: []string{"Ndp1"},
		Usage:   "First percentile for delivery latency (e.g., 0.1 for P10). Use with --network-latency-delivery-p1-value, --network-latency-delivery-p2, --network-latency-delivery-p2-value",
		Value:   0,
	}
	networkLatencyDeliveryP1ValueFlag = &cli.DurationFlag{
		Name:    "network-latency-delivery-p1-value",
		Aliases: []string{"Ndp1v"},
		Usage:   "Target value for delivery latency p1. Use with --network-latency-delivery-p1, --network-latency-delivery-p2, --network-latency-delivery-p2-value",
		Value:   0,
	}
	networkLatencyDeliveryP2Flag = &cli.Float64Flag{
		Name:    "network-latency-delivery-p2",
		Aliases: []string{"Ndp2"},
		Usage:   "Second percentile for delivery latency (e.g., 0.95 for P95). Use with --network-latency-delivery-p1, --network-latency-delivery-p1-value, --network-latency-delivery-p2-value",
		Value:   0,
	}
	networkLatencyDeliveryP2ValueFlag = &cli.DurationFlag{
		Name:    "network-latency-delivery-p2-value",
		Aliases: []string{"Ndp2v"},
		Usage:   "Target value for delivery latency p2. Use with --network-latency-delivery-p1, --network-latency-delivery-p1-value, --network-latency-delivery-p2",
		Value:   0,
	}
	stateDelayModelFlag = &cli.StringFlag{
		Name:    "state-delay-model",
		Aliases: []string{"Sdm"},
		Usage:   "State processing delay model to use (none, fixed, sampled)",
		Value:   "fitted",
	}
	stateDelayPresetFlag = &cli.StringFlag{
		Name:    "state-delay-preset",
		Aliases: []string{"Sdp"},
		Usage:   "State delay preset to use (fast, slow, noisy). Overrides individual p1/p2 flags when used with sampled model.",
		Value:   "",
	}
	stateDelayFixedTransactionFlag = &cli.DurationFlag{
		Name:    "state-delay-fixed-transaction",
		Aliases: []string{"Sft"},
		Usage:   "Fixed transaction processing delay (only used with fixed delay model)",
		Value:   0,
	}
	stateDelayFixedFinalizationFlag = &cli.DurationFlag{
		Name:    "state-delay-fixed-finalization",
		Aliases: []string{"Sff"},
		Usage:   "Fixed block finalization delay (only used with fixed delay model)",
		Value:   0,
	}
	// Sampled transaction delay - Two Percentiles
	stateDelayTransactionP1Flag = &cli.Float64Flag{
		Name:    "state-delay-transaction-p1",
		Aliases: []string{"Stp1"},
		Usage:   "First percentile for transaction delay (e.g., 0.1 for P10). Use with --state-delay-transaction-p1-value, --state-delay-transaction-p2, --state-delay-transaction-p2-value",
		Value:   0,
	}
	stateDelayTransactionP1ValueFlag = &cli.DurationFlag{
		Name:    "state-delay-transaction-p1-value",
		Aliases: []string{"Stp1v"},
		Usage:   "Target value for transaction delay p1. Use with --state-delay-transaction-p1, --state-delay-transaction-p2, --state-delay-transaction-p2-value",
		Value:   0,
	}
	stateDelayTransactionP2Flag = &cli.Float64Flag{
		Name:    "state-delay-transaction-p2",
		Aliases: []string{"Stp2"},
		Usage:   "Second percentile for transaction delay (e.g., 0.95 for P95). Use with --state-delay-transaction-p1, --state-delay-transaction-p1-value, --state-delay-transaction-p2-value",
		Value:   0,
	}
	stateDelayTransactionP2ValueFlag = &cli.DurationFlag{
		Name:    "state-delay-transaction-p2-value",
		Aliases: []string{"Stp2v"},
		Usage:   "Target value for transaction delay p2. Use with --state-delay-transaction-p1, --state-delay-transaction-p1-value, --state-delay-transaction-p2",
		Value:   0,
	}
	// Sampled finalization delay - Two Percentiles
	stateDelayFinalizationP1Flag = &cli.Float64Flag{
		Name:    "state-delay-finalization-p1",
		Aliases: []string{"Sfp1"},
		Usage:   "First percentile for finalization delay (e.g., 0.1 for P10). Use with --state-delay-finalization-p1-value, --state-delay-finalization-p2, --state-delay-finalization-p2-value",
		Value:   0,
	}
	stateDelayFinalizationP1ValueFlag = &cli.DurationFlag{
		Name:    "state-delay-finalization-p1-value",
		Aliases: []string{"Sfp1v"},
		Usage:   "Target value for finalization delay p1. Use with --state-delay-finalization-p1, --state-delay-finalization-p2, --state-delay-finalization-p2-value",
		Value:   0,
	}
	stateDelayFinalizationP2Flag = &cli.Float64Flag{
		Name:    "state-delay-finalization-p2",
		Aliases: []string{"Sfp2"},
		Usage:   "Second percentile for finalization delay (e.g., 0.95 for P95). Use with --state-delay-finalization-p1, --state-delay-finalization-p1-value, --state-delay-finalization-p2-value",
		Value:   0,
	}
	stateDelayFinalizationP2ValueFlag = &cli.DurationFlag{
		Name:    "state-delay-finalization-p2-value",
		Aliases: []string{"Sfp2v"},
		Usage:   "Target value for finalization delay p2. Use with --state-delay-finalization-p1, --state-delay-finalization-p1-value, --state-delay-finalization-p2",
		Value:   0,
	}
)

// getEvalCommand assembles the "eval" sub-command of the Daphne application
// intended to evaluate scenarios and collect performance data for post-mortem
// analysis.
func getEvalCommand() *cli.Command {
	return &cli.Command{
		Name:   "eval",
		Usage:  "Runs a simulation and collects performance data",
		Action: evalAction,
		Flags: []cli.Flag{
			durationFlag,
			numValidatorsFlag,
			numRpcNodesFlag,
			numObserversFlag,
			outputFileFlag,
			realTimeFlag,
			txPerSecondFlag,
			broadcastProtocolFlag,
			consensusProtocolFlag,
			topologyFlag,
			topologyNFlag,
			topologySeedFlag,
			networkLatencyModelFlag,
			networkLatencyPresetFlag,
			networkLatencyFixedSendFlag,
			networkLatencyFixedDeliveryFlag,
			networkLatencySendP1Flag,
			networkLatencySendP1ValueFlag,
			networkLatencySendP2Flag,
			networkLatencySendP2ValueFlag,
			networkLatencyDeliveryP1Flag,
			networkLatencyDeliveryP1ValueFlag,
			networkLatencyDeliveryP2Flag,
			networkLatencyDeliveryP2ValueFlag,
			stateDelayModelFlag,
			stateDelayPresetFlag,
			stateDelayFixedTransactionFlag,
			stateDelayFixedFinalizationFlag,
			stateDelayTransactionP1Flag,
			stateDelayTransactionP1ValueFlag,
			stateDelayTransactionP2Flag,
			stateDelayTransactionP2ValueFlag,
			stateDelayFinalizationP1Flag,
			stateDelayFinalizationP1ValueFlag,
			stateDelayFinalizationP2Flag,
			stateDelayFinalizationP2ValueFlag,
		},
	}
}

func evalAction(ctx context.Context, c *cli.Command) error {
	scenario, err := loadScenario(c)
	if err != nil {
		return err
	}
	return runScenario(parseRunConfig(c), scenario)
}

func loadScenario(c *cli.Command) (scenario.Scenario, error) {
	// For now, this command runs a simple place-holder scenario. In the future,
	// this scenario should be replaced by a scripted setup.

	numValidators := c.Int(numValidatorsFlag.Name)
	if numValidators <= 0 {
		return nil, fmt.Errorf("number of validators must be positive, got %d", numValidators)
	}

	numRpcNodes := c.Int(numRpcNodesFlag.Name)
	if numRpcNodes <= 0 {
		return nil, fmt.Errorf("number of RPC nodes must be positive, got %d", numRpcNodes)
	}

	numObservers := c.Int(numObserversFlag.Name)
	if numObservers < 0 {
		return nil, fmt.Errorf("number of observers cannot be negative, got %d", numObservers)
	}

	broadcastProtocol := getBroadcastProtocol(c.String(broadcastProtocolFlag.Name))

	networkGeography, err := getNetworkGeography(c)
	if err != nil {
		return nil, fmt.Errorf("failed to configure network latency model: %w", err)
	}

	stateDelayModel, err := getStateDelayModel(c)
	if err != nil {
		return nil, fmt.Errorf("failed to configure state delay model: %w", err)
	}

	return &scenario.DemoScenario{
		NumValidators: numValidators,
		NumRpcNodes:   numRpcNodes,
		NumObservers:  numObservers,
		TxPerSecond:   c.Int(txPerSecondFlag.Name),
		Duration:      c.Duration(durationFlag.Name),
		Broadcast:     broadcastProtocol,
		Consensus: getConsensusFactory(
			c.String(consensusProtocolFlag.Name),
			broadcastProtocol,
		),
		Topology:                  getNetworkTopology(c),
		NetworkGeography:          networkGeography,
		StateProcessingDelayModel: stateDelayModel,
	}, nil
}

func getBroadcastProtocol(protocol string) broadcast.Protocol {
	switch strings.ToLower(protocol) {
	case "forwarding", "f":
		slog.Info("Using forwarding broadcast protocol")
		return broadcast.ProtocolForwarding
	case "gossip", "g":
		slog.Info("Using gossip broadcast protocol")
		return broadcast.ProtocolGossip
	default:
		slog.Warn("Unknown broadcast protocol in configuration, using gossip protocol", "unknown_protocol", protocol)
		return broadcast.ProtocolGossip
	}
}

func getConsensusFactory(
	protocol string,
	broadcastProtocol broadcast.Protocol,
) consensus.Factory {
	switch strings.ToLower(protocol) {
	case "central", "c":
		slog.Info("Using central consensus protocol")
		return central.Factory{
			EmitInterval:     500 * time.Millisecond,
			BroadcastFactory: broadcast.GetFactory[uint32, central.BundleMessage](broadcastProtocol),
		}
	case "streamlet", "s":
		slog.Info("Using streamlet consensus protocol")
		return streamlet.Factory{
			EpochDuration: 500 * time.Millisecond,
		}
	case "autocrat", "a":
		slog.Info("Using autocrat consensus protocol")
		return dag.Factory[payload.Transactions]{
			LayeringFactory:        autocracy.Factory{},
			PayloadProtocolFactory: payload.RawProtocolFactory{},
		}
	case "lachesis", "l":
		slog.Info("Using lachesis consensus protocol")
		return dag.Factory[payload.Transactions]{
			LayeringFactory:        moira.LachesisFactory{},
			PayloadProtocolFactory: payload.RawProtocolFactory{},
		}
	default:
		slog.Warn("Unknown consensus protocol in configuration, using defaults", "unknown_protocol", protocol)
		return getConsensusFactory("central", broadcastProtocol)
	}
}

func getNetworkTopology(c *cli.Command) p2p.TopologyFactory {
	topology := strings.ToLower(c.String(topologyFlag.Name))

	switch topology {
	case "fully-meshed", "f":
		slog.Info("Using fully-meshed network topology")
		return p2p.FullyMeshedTopologyFactory{}
	case "line", "l":
		slog.Info("Using line network topology")
		return p2p.LineTopologyFactory{}
	case "ring", "r":
		slog.Info("Using ring network topology")
		return p2p.RingTopologyFactory{}
	case "star", "s":
		slog.Info("Using star network topology")
		return p2p.StarTopologyFactory{}
	case "random", "rand":
		n := c.Int(topologyNFlag.Name)
		seed := c.Int64(topologySeedFlag.Name)
		slog.Info("Using random n-ary graph topology", "n", n, "seed", seed)
		return p2p.RandomNaryGraphTopologyFactory{N: n, Seed: seed}
	default:
		slog.Warn(
			"Unknown network topology, using fully-meshed",
			"unknown_topology",
			topology,
		)
		return p2p.FullyMeshedTopologyFactory{}
	}
}

// RunConfig holds the configuration options for running a simulation.
type RunConfig struct {
	outputFile string
	useSimTime bool
}

// parseRunConfig parses the command-line flags and returns a RunConfig
// struct with the corresponding settings.
func parseRunConfig(c *cli.Command) RunConfig {
	return RunConfig{
		outputFile: c.String(outputFileFlag.Name),
		useSimTime: !c.Bool(realTimeFlag.Name),
	}
}

func getNetworkGeography(c *cli.Command) (scenario.NetworkGeography, error) {
	modelType := strings.ToLower(c.String(networkLatencyModelFlag.Name))

	switch modelType {
	case "none", "":
		slog.Info("Using no network latency model")
		return scenario.NewSimpleNetworkGeography(nil, nil), nil

	case "fixed", "f":
		slog.Info("Using fixed network latency model")

		sendLatency := c.Duration(networkLatencyFixedSendFlag.Name)
		deliveryLatency := c.Duration(networkLatencyFixedDeliveryFlag.Name)

		if sendLatency > 0 {
			slog.Info("Fixed send latency configured", "latency", sendLatency)
		}

		if deliveryLatency > 0 {
			slog.Info("Fixed delivery latency configured", "latency", deliveryLatency)
		}

		return scenario.NewSimpleNetworkGeography(utils.FixedDelay(sendLatency), utils.FixedDelay(deliveryLatency)), nil

	case "sampled", "s":
		slog.Info("Using sampled network latency model")

		// Get percentile values either from preset or from flags
		preset := strings.ToLower(c.String(networkLatencyPresetFlag.Name))
		sendP1, sendP1Value, sendP2, sendP2Value, deliveryP1, deliveryP1Value,
			deliveryP2, deliveryP2Value := getLatencyValues(c, preset)

		// Configure send latency distribution
		sendDist, err := configureSampledLatency(
			"send",
			sendP1,
			sendP1Value,
			sendP2,
			sendP2Value,
		)
		if err != nil {
			return scenario.NetworkGeography{}, fmt.Errorf("failed to configure send latency distribution: %w", err)
		}

		// Configure delivery latency distribution
		deliveryDist, err := configureSampledLatency(
			"delivery",
			deliveryP1,
			deliveryP1Value,
			deliveryP2,
			deliveryP2Value,
		)
		if err != nil {
			return scenario.NetworkGeography{}, fmt.Errorf("failed to configure delivery latency distribution: %w", err)
		}

		return scenario.NewSimpleNetworkGeography(sendDist, deliveryDist), nil

	default:
		return scenario.NetworkGeography{}, fmt.Errorf("unknown network latency model: %s", modelType)
	}
}

// getLatencyValues returns the latency percentile values either from a preset
// or from flags
func getLatencyValues(c *cli.Command, preset string) (
	sendP1 float64, sendP1Value time.Duration, sendP2 float64, sendP2Value time.Duration,
	deliveryP1 float64, deliveryP1Value time.Duration, deliveryP2 float64, deliveryP2Value time.Duration,
) {
	switch preset {
	case "fast":
		// Fast network: low latency, tight distribution
		// P10 = 5ms, P90 = 15ms for send
		// P10 = 2ms, P90 = 8ms for delivery
		slog.Info("Using 'fast' network latency preset")
		return 0.1, 5 * time.Millisecond, 0.9, 15 * time.Millisecond,
			0.1, 2 * time.Millisecond, 0.9, 8 * time.Millisecond

	case "slow":
		// Slow network: higher latency, moderate distribution
		// P10 = 50ms, P90 = 150ms for send
		// P10 = 30ms, P90 = 100ms for delivery
		slog.Info("Using 'slow' network latency preset")
		return 0.1, 50 * time.Millisecond, 0.9, 150 * time.Millisecond,
			0.1, 30 * time.Millisecond, 0.9, 100 * time.Millisecond

	case "noisy":
		// Noisy network: long tail with P95 significantly higher than P50
		// P10 = 10ms, P95 = 500ms for send (long tail)
		// P10 = 5ms, P95 = 300ms for delivery (long tail)
		slog.Info("Using 'noisy' network latency preset with long tail")
		return 0.1, 10 * time.Millisecond, 0.95, 500 * time.Millisecond,
			0.1, 5 * time.Millisecond, 0.95, 300 * time.Millisecond

	default:
		// Use flag values if no preset specified
		return c.Float64(networkLatencySendP1Flag.Name),
			c.Duration(networkLatencySendP1ValueFlag.Name),
			c.Float64(networkLatencySendP2Flag.Name),
			c.Duration(networkLatencySendP2ValueFlag.Name),
			c.Float64(networkLatencyDeliveryP1Flag.Name),
			c.Duration(networkLatencyDeliveryP1ValueFlag.Name),
			c.Float64(networkLatencyDeliveryP2Flag.Name),
			c.Duration(networkLatencyDeliveryP2ValueFlag.Name)
	}
}

func configureSampledLatency(
	name string,
	p1 float64,
	p1Value time.Duration,
	p2 float64,
	p2Value time.Duration,
) (utils.Distribution, error) {
	// Validate that all required parameters are provided
	if p1 <= 0 || p1Value <= 0 || p2 <= 0 || p2Value <= 0 {
		return nil, fmt.Errorf(
			"%s: must specify all percentile parameters (p1, p1-value, p2, p2-value)",
			name,
		)
	}

	// Create distribution from two percentiles
	dist, err := utils.NewFromTwoPercentiles(
		p1,
		p1Value,
		p2,
		p2Value,
		time.Nanosecond,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("%s two-percentile configuration: %w", name, err)
	}

	slog.Info(
		fmt.Sprintf("Sampled %s configured (two percentiles)", name),
		"p1", p1,
		"p1_value", p1Value,
		"p2", p2,
		"p2_value", p2Value,
	)

	return dist, nil
}

func getStateDelayModel(c *cli.Command) (state.ProcessingDelayModel, error) {
	modelType := strings.ToLower(c.String(stateDelayModelFlag.Name))

	switch modelType {
	case "fitted", "":
		slog.Info("Using fitted state processing delay model")
		return getDefaultStateProcessingLatencyModel(), nil

	case "none":
		slog.Info("Using no state processing delay model")
		return nil, nil

	case "fixed", "f":
		slog.Info("Using fixed state processing delay model")
		model := state.NewFixedProcessingDelayModel()

		txDelay := c.Duration(stateDelayFixedTransactionFlag.Name)
		finalizationDelay := c.Duration(stateDelayFixedFinalizationFlag.Name)

		if txDelay > 0 {
			model.SetBaseTransactionDelay(txDelay)
			slog.Info("Fixed transaction delay configured", "delay", txDelay)
		}
		if finalizationDelay > 0 {
			model.SetBaseBlockFinalizationDelay(finalizationDelay)
			slog.Info("Fixed finalization delay configured", "delay", finalizationDelay)
		}

		return model, nil

	case "sampled", "s":
		slog.Info("Using sampled state processing delay model")
		model := state.NewSampledProcessingDelayModel()

		// Get percentile values either from preset or from flags
		preset := strings.ToLower(c.String(stateDelayPresetFlag.Name))
		txP1, txP1Value, txP2, txP2Value, finalizationP1, finalizationP1Value,
			finalizationP2, finalizationP2Value := getStateDelayValues(c, preset)

		// Configure transaction delay distribution
		txDist, err := configureSampledLatency(
			"transaction",
			txP1,
			txP1Value,
			txP2,
			txP2Value,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to configure transaction delay distribution: %w", err)
		}
		model.SetBaseTransactionDistribution(txDist)

		// Configure finalization delay distribution
		finalizationDist, err := configureSampledLatency(
			"finalization",
			finalizationP1,
			finalizationP1Value,
			finalizationP2,
			finalizationP2Value,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to configure finalization delay distribution: %w", err)
		}
		model.SetBaseBlockFinalizationDistribution(finalizationDist)

		return model, nil

	default:
		return nil, fmt.Errorf("unknown state delay model: %s", modelType)
	}
}

// getStateDelayValues returns the state delay percentile values either from a
// preset or from flags
func getStateDelayValues(c *cli.Command, preset string) (
	txP1 float64, txP1Value time.Duration, txP2 float64, txP2Value time.Duration,
	finalizationP1 float64, finalizationP1Value time.Duration, finalizationP2 float64, finalizationP2Value time.Duration,
) {
	switch preset {
	case "fast":
		// Fast processing: low delay, tight distribution
		// P10 = 1ms, P90 = 3ms for transaction
		// P10 = 2ms, P90 = 6ms for finalization
		slog.Info("Using 'fast' state delay preset")
		return 0.1, 1 * time.Millisecond, 0.9, 3 * time.Millisecond,
			0.1, 2 * time.Millisecond, 0.9, 6 * time.Millisecond

	case "slow":
		// Slow processing: higher delay, moderate distribution
		// P10 = 5ms, P90 = 20ms for transaction
		// P10 = 10ms, P90 = 50ms for finalization
		slog.Info("Using 'slow' state delay preset")
		return 0.1, 5 * time.Millisecond, 0.9, 20 * time.Millisecond,
			0.1, 10 * time.Millisecond, 0.9, 50 * time.Millisecond

	case "noisy":
		// Noisy processing: long tail with P95 significantly higher than P50
		// P10 = 1ms, P95 = 100ms for transaction (long tail)
		// P10 = 2ms, P95 = 200ms for finalization (long tail)
		slog.Info("Using 'noisy' state delay preset with long tail")
		return 0.1, 1 * time.Millisecond, 0.95, 100 * time.Millisecond,
			0.1, 2 * time.Millisecond, 0.95, 200 * time.Millisecond

	default:
		// Use flag values if no preset specified
		return c.Float64(stateDelayTransactionP1Flag.Name),
			c.Duration(stateDelayTransactionP1ValueFlag.Name),
			c.Float64(stateDelayTransactionP2Flag.Name),
			c.Duration(stateDelayTransactionP2ValueFlag.Name),
			c.Float64(stateDelayFinalizationP1Flag.Name),
			c.Duration(stateDelayFinalizationP1ValueFlag.Name),
			c.Float64(stateDelayFinalizationP2Flag.Name),
			c.Duration(stateDelayFinalizationP2ValueFlag.Name)
	}
}

// runScenario executes the given scenario using the user-selected execution
// mode (real-time or sim-time) and handles any errors that occur.
func runScenario(
	config RunConfig,
	scenario scenario.Scenario,
) (err error) {
	outputFile := config.outputFile
	slog.Info("Results will be saved to", "file", outputFile)

	sink, err := tracker.NewParquetSink(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create parquet sink: %w", err)
	}
	root := tracker.New(sink)
	defer func() {
		slog.Info("Flushing results to disk")
		err = errors.Join(err, root.Close())
	}()
	err = runScenarioWithTracker(config, scenario, root)
	if err != nil {
		return err
	}
	return nil
}

func runScenarioWithTracker(
	config RunConfig,
	scenario scenario.Scenario,
	tracker tracker.Tracker,
) error {
	run := runRealTime
	mode := "real-time"
	if config.useSimTime {
		mode = "simulated-time"
		run = runSimTime
	}

	slog.Info("Running scenario", "mode", mode)

	err := run(tracker, scenario)
	if err != nil {
		slog.Error("Failed to run simulation", "error", err)
		return err
	}
	return nil
}

func runRealTime(
	tracker tracker.Tracker,
	scenario scenario.Scenario,
) error {
	return scenario.Run(slog.Default(), tracker)
}

func runSimTime(
	tracker tracker.Tracker,
	scenario scenario.Scenario,
) error {
	var err error
	synctest.Test(&testing.T{}, func(*testing.T) {
		err = scenario.Run(slog.Default(), tracker)
		// Settlement of threads.
		time.Sleep(1 * time.Hour)
	})
	return err
}
