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
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/lachesis"
	"github.com/0xsoniclabs/daphne/daphne/consensus/streamlet"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/sim/scenario"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/types"
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
	numNodesFlag = &cli.IntFlag{
		Name:    "num-nodes",
		Aliases: []string{"n"},
		Usage:   "Number of nodes in the simulation",
		Value:   3,
	}
	txPerSecondFlag = &cli.IntFlag{
		Name:    "tx-per-second",
		Aliases: []string{"t"},
		Usage:   "Number of transactions per second",
		Value:   100,
	}
	simTimeFlag = &cli.BoolFlag{
		Name:    "sim-time",
		Aliases: []string{"s"},
		Usage:   "Run the simulation in simulated time mode",
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
	// Sampled send latency - Method 1: Median + Percentile
	networkLatencySendMedianFlag = &cli.DurationFlag{
		Name:    "network-latency-send-median",
		Aliases: []string{"Nsm"},
		Usage:   "Median send latency (P50). Use with --network-latency-send-percentile and --network-latency-send-percentile-value",
		Value:   0,
	}
	networkLatencySendPercentileFlag = &cli.Float64Flag{
		Name:    "network-latency-send-percentile",
		Aliases: []string{"Nsp"},
		Usage:   "Percentile for send latency (e.g., 0.95 for P95). Use with --network-latency-send-median and --network-latency-send-percentile-value",
		Value:   0,
	}
	networkLatencySendPercentileValueFlag = &cli.DurationFlag{
		Name:    "network-latency-send-percentile-value",
		Aliases: []string{"Nspv"},
		Usage:   "Target value for send latency percentile. Use with --network-latency-send-median and --network-latency-send-percentile",
		Value:   0,
	}
	// Sampled send latency - Method 2: Two Percentiles
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
	// Sampled delivery latency - Method 1: Median + Percentile
	networkLatencyDeliveryMedianFlag = &cli.DurationFlag{
		Name:    "network-latency-delivery-median",
		Aliases: []string{"Ndm"},
		Usage:   "Median delivery latency (P50). Use with --network-latency-delivery-percentile and --network-latency-delivery-percentile-value",
		Value:   0,
	}
	networkLatencyDeliveryPercentileFlag = &cli.Float64Flag{
		Name:    "network-latency-delivery-percentile",
		Aliases: []string{"Ndp"},
		Usage:   "Percentile for delivery latency (e.g., 0.95 for P95). Use with --network-latency-delivery-median and --network-latency-delivery-percentile-value",
		Value:   0,
	}
	networkLatencyDeliveryPercentileValueFlag = &cli.DurationFlag{
		Name:    "network-latency-delivery-percentile-value",
		Aliases: []string{"Ndpv"},
		Usage:   "Target value for delivery latency percentile. Use with --network-latency-delivery-median and --network-latency-delivery-percentile",
		Value:   0,
	}
	// Sampled delivery latency - Method 2: Two Percentiles
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
)

// getRunCommand assembles the "run" sub-command of the Daphne application
// intended to run scenarios and collect performance data for post-mortem
// analysis.
func getRunCommand() *cli.Command {
	return &cli.Command{
		Name:   "run",
		Usage:  "Runs a simulation and collects performance data",
		Action: runAction,
		Flags: []cli.Flag{
			durationFlag,
			numNodesFlag,
			outputFileFlag,
			simTimeFlag,
			txPerSecondFlag,
			broadcastProtocolFlag,
			consensusProtocolFlag,
			topologyFlag,
			topologyNFlag,
			topologySeedFlag,
			networkLatencyModelFlag,
			networkLatencyFixedSendFlag,
			networkLatencyFixedDeliveryFlag,
			networkLatencySendMedianFlag,
			networkLatencySendPercentileFlag,
			networkLatencySendPercentileValueFlag,
			networkLatencySendP1Flag,
			networkLatencySendP1ValueFlag,
			networkLatencySendP2Flag,
			networkLatencySendP2ValueFlag,
			networkLatencyDeliveryMedianFlag,
			networkLatencyDeliveryPercentileFlag,
			networkLatencyDeliveryPercentileValueFlag,
			networkLatencyDeliveryP1Flag,
			networkLatencyDeliveryP1ValueFlag,
			networkLatencyDeliveryP2Flag,
			networkLatencyDeliveryP2ValueFlag,
		},
	}
}

func runAction(ctx context.Context, c *cli.Command) error {
	scenario, err := loadScenario(c)
	if err != nil {
		return err
	}
	return runScenario(parseRunConfig(c), scenario)
}

func loadScenario(c *cli.Command) (scenario.Scenario, error) {
	// For now, this command runs a simple place-holder scenario. In the future,
	// this scenario should be replaced by a scripted setup.

	numNodes := c.Int(numNodesFlag.Name)
	if numNodes <= 0 {
		return nil, fmt.Errorf("number of nodes must be positive, got %d", numNodes)
	}

	stakes := make(map[consensus.ValidatorId]uint32)
	for i := 0; i < numNodes; i++ {
		stakes[consensus.ValidatorId(i)] = 1
	}
	committee, _ := consensus.NewCommittee(stakes) // only fails on empty stakes

	broadcastFactories := getBroadcastFactories(c.String(broadcastProtocolFlag.Name))

	latencyModel, err := getNetworkLatencyModel(c)
	if err != nil {
		return nil, fmt.Errorf("failed to configure network latency model: %w", err)
	}

	return &scenario.DemoScenario{
		NumNodes:    numNodes,
		TxPerSecond: c.Int(txPerSecondFlag.Name),
		Duration:    c.Duration(durationFlag.Name),
		Broadcaster: broadcastFactories,
		Consensus: getConsensusFactory(
			c.String(consensusProtocolFlag.Name),
			*committee,
			broadcastFactories,
		),
		Topology:            getNetworkTopology(c, numNodes),
		NetworkLatencyModel: latencyModel,
	}, nil
}

func getBroadcastFactories(protocol string) broadcast.Factories {
	factory := broadcast.Factories{}
	switch strings.ToLower(protocol) {
	case "forwarding", "f":
		slog.Info("Using forwarding broadcast protocol")
		factory = broadcast.SetFactory(factory, broadcast.NewForwarding[types.Hash, types.Transaction])
		factory = broadcast.SetFactory(factory, broadcast.NewForwarding[uint32, central.BundleMessage])
		return factory
	case "gossip", "g":
		slog.Info("Using gossip broadcast protocol")
		factory = broadcast.SetFactory(factory, broadcast.NewGossip[types.Hash, types.Transaction])
		factory = broadcast.SetFactory(factory, broadcast.NewGossip[uint32, central.BundleMessage])
		return factory
	default:
		slog.Warn("Unknown broadcast protocol in configuration, using defaults", "unknown_protocol", protocol)
		return factory
	}
}

func getConsensusFactory(
	protocol string,
	committee consensus.Committee,
	broadcastFactories broadcast.Factories,
) consensus.Factory {
	switch strings.ToLower(protocol) {
	case "central", "c":
		slog.Info("Using central consensus protocol")
		return central.Factory{
			EmitInterval:     500 * time.Millisecond,
			Leader:           p2p.PeerId("N-001"),
			BroadcastFactory: broadcast.GetFactory[uint32, central.BundleMessage](broadcastFactories),
		}
	case "streamlet", "s":
		slog.Info("Using streamlet consensus protocol")
		return streamlet.Factory{
			EpochDuration: 500 * time.Millisecond,
			Committee:     committee,
		}
	case "autocrat", "a":
		slog.Info("Using autocrat consensus protocol")
		return dag.Factory{
			LayeringFactory: autocracy.Factory{},
			Committee:       &committee,
		}
	case "lachesis", "l":
		slog.Info("Using lachesis consensus protocol")
		return dag.Factory{
			LayeringFactory: lachesis.Factory{},
			Committee:       &committee,
		}
	default:
		slog.Warn("Unknown consensus protocol in configuration, using defaults", "unknown_protocol", protocol)
		return getConsensusFactory("central", committee, broadcastFactories)
	}
}

func getNetworkTopology(c *cli.Command, numNodes int) p2p.NetworkTopology {
	topology := strings.ToLower(c.String(topologyFlag.Name))

	peerIds := make([]p2p.PeerId, numNodes)
	for i := range numNodes {
		peerIds[i] = p2p.PeerId(fmt.Sprintf("N-%03d", i+1))
	}

	switch topology {
	case "fully-meshed", "f":
		slog.Info("Using fully-meshed network topology")
		return p2p.NewFullyMeshedTopology()
	case "line", "l":
		slog.Info("Using line network topology")
		return p2p.NewLineTopology(peerIds)
	case "ring", "r":
		slog.Info("Using ring network topology")
		return p2p.NewRingTopology(peerIds)
	case "star", "s":
		// Use the first node as the hub
		hub := peerIds[0]
		slog.Info("Using star network topology", "hub", hub)
		return p2p.NewStarTopology(hub, peerIds)
	case "random", "rand":
		n := c.Int(topologyNFlag.Name)
		seed := c.Int64(topologySeedFlag.Name)
		slog.Info("Using random n-ary graph topology", "n", n, "seed", seed)
		return p2p.NewRandomNaryGraphTopology(peerIds, n, seed)
	default:
		slog.Warn(
			"Unknown network topology, using fully-meshed",
			"unknown_topology",
			topology,
		)
		return p2p.NewFullyMeshedTopology()
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
		useSimTime: c.Bool(simTimeFlag.Name),
	}
}

func getNetworkLatencyModel(c *cli.Command) (p2p.LatencyModel, error) {
	modelType := strings.ToLower(c.String(networkLatencyModelFlag.Name))

	switch modelType {
	case "none", "":
		slog.Info("Using no network latency model")
		return nil, nil

	case "fixed", "f":
		slog.Info("Using fixed network latency model")
		model := p2p.NewFixedDelayModel()

		sendLatency := c.Duration(networkLatencyFixedSendFlag.Name)
		deliveryLatency := c.Duration(networkLatencyFixedDeliveryFlag.Name)

		if sendLatency > 0 {
			model.SetBaseSendDelay(sendLatency)
			slog.Info("Fixed send latency configured", "latency", sendLatency)
		}
		if deliveryLatency > 0 {
			model.SetBaseDeliveryDelay(deliveryLatency)
			slog.Info("Fixed delivery latency configured", "latency", deliveryLatency)
		}

		return model, nil

	case "sampled", "s":
		slog.Info("Using sampled network latency model")
		model := p2p.NewSampledDelayModel()

		// Configure send latency distribution
		if err := configureSampledLatency(
			c,
			"send",
			networkLatencySendMedianFlag.Name,
			networkLatencySendPercentileFlag.Name,
			networkLatencySendPercentileValueFlag.Name,
			networkLatencySendP1Flag.Name,
			networkLatencySendP1ValueFlag.Name,
			networkLatencySendP2Flag.Name,
			networkLatencySendP2ValueFlag.Name,
			func(dist utils.Distribution) {
				model.SetBaseSendDistribution(dist)
			},
		); err != nil {
			return nil, fmt.Errorf("failed to configure send latency distribution: %w", err)
		}

		// Configure delivery latency distribution
		if err := configureSampledLatency(
			c,
			"delivery",
			networkLatencyDeliveryMedianFlag.Name,
			networkLatencyDeliveryPercentileFlag.Name,
			networkLatencyDeliveryPercentileValueFlag.Name,
			networkLatencyDeliveryP1Flag.Name,
			networkLatencyDeliveryP1ValueFlag.Name,
			networkLatencyDeliveryP2Flag.Name,
			networkLatencyDeliveryP2ValueFlag.Name,
			func(dist utils.Distribution) {
				model.SetBaseDeliveryDistribution(dist)
			},
		); err != nil {
			return nil, fmt.Errorf("failed to configure delivery latency distribution: %w", err)
		}

		return model, nil

	default:
		return nil, fmt.Errorf("unknown network latency model: %s", modelType)
	}
}

func configureSampledLatency(
	c *cli.Command,
	name string,
	medianFlagName string,
	percentileFlagName string,
	percentileValueFlagName string,
	p1FlagName string,
	p1ValueFlagName string,
	p2FlagName string,
	p2ValueFlagName string,
	setter func(utils.Distribution),
) error {
	median := c.Duration(medianFlagName)
	percentile := c.Float64(percentileFlagName)
	percentileValue := c.Duration(percentileValueFlagName)
	p1 := c.Float64(p1FlagName)
	p1Value := c.Duration(p1ValueFlagName)
	p2 := c.Float64(p2FlagName)
	p2Value := c.Duration(p2ValueFlagName)

	// Determine which constructor to use
	if median > 0 {
		// Method 1: Use NewFromMedianAndPercentile
		if percentile <= 0 || percentileValue <= 0 {
			return fmt.Errorf(
				"%s latency: median specified but -percentile or -percentile-value not provided",
				name,
			)
		}

		dist, err := utils.NewFromMedianAndPercentile(
			median,
			percentile,
			percentileValue,
			time.Nanosecond,
			nil,
		)
		if err != nil {
			return fmt.Errorf("%s latency median+percentile configuration: %w", name, err)
		}

		setter(dist)
		slog.Info(
			fmt.Sprintf("Sampled %s latency configured (median+percentile)", name),
			"median", median,
			"percentile", percentile,
			"percentile_value", percentileValue,
		)
	} else {
		// Method 2: Use NewFromTwoPercentiles
		if p1 <= 0 || p1Value <= 0 || p2 <= 0 || p2Value <= 0 {
			return fmt.Errorf(
				"%s latency: must specify either (median + percentile + percentile-value) OR (p1 + p1-value + p2 + p2-value)",
				name,
			)
		}

		dist, err := utils.NewFromTwoPercentiles(
			p1,
			p1Value,
			p2,
			p2Value,
			time.Nanosecond,
			nil,
		)
		if err != nil {
			return fmt.Errorf("%s latency two-percentile configuration: %w", name, err)
		}

		setter(dist)
		slog.Info(
			fmt.Sprintf("Sampled %s latency configured (two percentiles)", name),
			"p1", p1,
			"p1_value", p1Value,
			"p2", p2,
			"p2_value", p2Value,
		)
	}

	return nil
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
	})
	return err
}
