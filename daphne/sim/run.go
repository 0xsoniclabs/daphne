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
		Topology: getNetworkTopology(c, numNodes),
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
