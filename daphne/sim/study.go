package sim

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/central"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/autocracy"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/lachesis"
	"github.com/0xsoniclabs/daphne/daphne/consensus/streamlet"
	"github.com/0xsoniclabs/daphne/daphne/consensus/tendermint"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/sim/scenario"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/urfave/cli/v3"
	"golang.org/x/exp/constraints"
)

// --- Study command ---

var (
	studyOutputFile = &cli.StringFlag{
		Name:    outputFileFlag.Name,
		Aliases: outputFileFlag.Aliases,
		Usage:   "Path to the output file for the study results",
		Value:   "data.parquet",
	}
	repetitionsFlag = &cli.IntFlag{
		Name:    "repetitions",
		Aliases: []string{"r"},
		Usage:   "Number of repetitions per scenario in the study",
		Value:   1,
	}
)

// getStudyCommand assembles the "study" sub-command of the Daphne application
// intended to run a range of simulation scenarios defined by a parameter study.
func getStudyCommand() *cli.Command {
	return &cli.Command{
		Name:  "study",
		Usage: "Runs parameter studies using the Daphne environment",
		Flags: []cli.Flag{
			durationFlag,
			realTimeFlag,
			studyOutputFile,
			repetitionsFlag,
		},
		Commands: []*cli.Command{
			{
				Name:   "load",
				Usage:  "Runs a load study varying the number of nodes and transaction rates",
				Action: loadStudyAction,
			},
			{
				Name:   "broadcast",
				Usage:  "Runs a study varying the broadcast protocol used in the simulation",
				Action: broadcastStudyAction,
			},
			{
				Name:   "consensus",
				Usage:  "Runs a study varying the consensus algorithm used in the simulation",
				Action: consensusStudyAction,
			},
		},
	}
}

// StudyConfig is a summary of the configuration parameters for running a
// parameter study. These options are typically parsed from CLI flags.
type StudyConfig struct {
	RunConfig
	duration    time.Duration
	repetitions int
}

func loadStudyAction(ctx context.Context, c *cli.Command) error {
	return studyAction(ctx, c, getLoadStudy())
}

func broadcastStudyAction(ctx context.Context, c *cli.Command) error {
	return studyAction(ctx, c, getBroadcastProtocolStudy())
}

func consensusStudyAction(ctx context.Context, c *cli.Command) error {
	return studyAction(ctx, c, getConsensusProtocolStudy())
}

func studyAction(
	ctx context.Context,
	c *cli.Command,
	study Study,
) error {
	config := StudyConfig{
		RunConfig:   parseRunConfig(c),
		duration:    c.Duration(durationFlag.Name),
		repetitions: max(1, c.Int(repetitionsFlag.Name)),
	}
	if config.duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	return _studyAction(
		ctx,
		config,
		study,
		func(scenario scenario.Scenario, tracker tracker.Tracker) error {
			return runScenarioWithTracker(config.RunConfig, scenario, tracker)
		},
	)
}

// _studyAction is an internal helper to run the parameter study with the given
// configuration, study definition, and scenario runner function. It is separated
// from studyAction to facilitate testing.
func _studyAction(
	ctx context.Context,
	config StudyConfig,
	study Study,
	run func(scenario scenario.Scenario, tracker tracker.Tracker) error,
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

	all := slices.Collect(study.All())
	slog.Info("Running study", "num_scenarios", len(all))
	runId := 0
	for repetition := range max(1, config.repetitions) {
		for i, scenario := range all {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Info("Running scenario",
				"repetition", repetition+1, "of", config.repetitions,
				"scenario", i+1, "of", len(all),
			)
			scenario.Duration = config.duration

			tracker := root.With("rid", runId)
			study.AddLabels(tracker, scenario).Track(mark.StudyStarted)
			runId++

			if err := run(&scenario, tracker); err != nil {
				return fmt.Errorf("failed scenario %d/%d: %w", i+1, len(all), err)
			}
		}
	}
	return nil
}

// --- Study definition ---

// getLoadStudy returns a parameter study definition varying the number of
// nodes and transaction rates using a default consensus algorithm, the gossip
// broadcast protocol, and fully meshed network topology.
func getLoadStudy() Study {
	return Study{
		Dimensions: []Dimension{
			Dim(NumNodes{}, Range(1, 21)),
			Dim(TxPerSecond{}, List(5, 10, 20)),
			Dim(Broadcast{}, List(broadcast.ProtocolGossip)),
			Dim(Consensus{}, List[consensus.Factory](
				central.Factory{},
			)),
			Dim(Topology{}, List[p2p.NetworkTopology](
				p2p.NewFullyMeshedTopology(),
			)),
		},
	}
}

// getBroadcastProtocolStudy returns a parameter study definition that varies the
// broadcasting protocol used in the simulation scenarios.
func getBroadcastProtocolStudy() Study {
	return Study{
		Dimensions: []Dimension{
			Dim(NumNodes{}, Range(1, 21)),
			Dim(TxPerSecond{}, List(10)),
			Dim(Broadcast{}, List(
				broadcast.ProtocolGossip,
				broadcast.ProtocolForwarding,
			)),
			Dim(Consensus{}, List[consensus.Factory](
				central.Factory{},
			)),
			Dim(Topology{}, List[p2p.NetworkTopology](
				p2p.NewFullyMeshedTopology(),
			)),
		},
	}
}

// getConsensusProtocolStudy returns a parameter study definition that varies
// the consensus algorithm used in the simulation scenarios.
func getConsensusProtocolStudy() Study {
	return Study{
		Dimensions: []Dimension{
			Dim(NumNodes{}, List(20)),
			Dim(TxPerSecond{}, List(100)),
			Dim(Broadcast{}, List(broadcast.ProtocolGossip)),
			Dim(Consensus{}, List[consensus.Factory](
				central.Factory{
					EmitInterval: 100 * time.Millisecond,
				},
				central.Factory{
					EmitInterval: 250 * time.Millisecond,
				},
				central.Factory{
					EmitInterval: 500 * time.Millisecond,
				},
				streamlet.Factory{
					EpochDuration: 100 * time.Millisecond,
				},
				streamlet.Factory{
					EpochDuration: 250 * time.Millisecond,
				},
				streamlet.Factory{
					EpochDuration: 500 * time.Millisecond,
				},
				tendermint.Factory{
					ProposePhaseTimeout:   100 * time.Millisecond,
					PrevotePhaseTimeout:   100 * time.Millisecond,
					PrecommitPhaseTimeout: 100 * time.Millisecond,
					PhaseTimeoutDelta:     10 * time.Millisecond,
				},
				tendermint.Factory{
					ProposePhaseTimeout:   250 * time.Millisecond,
					PrevotePhaseTimeout:   250 * time.Millisecond,
					PrecommitPhaseTimeout: 250 * time.Millisecond,
					PhaseTimeoutDelta:     10 * time.Millisecond,
				},
				tendermint.Factory{
					ProposePhaseTimeout:   500 * time.Millisecond,
					PrevotePhaseTimeout:   500 * time.Millisecond,
					PrecommitPhaseTimeout: 500 * time.Millisecond,
					PhaseTimeoutDelta:     10 * time.Millisecond,
				},
				dag.Factory{
					EmitInterval:    100 * time.Millisecond,
					LayeringFactory: autocracy.Factory{},
				},
				dag.Factory{
					EmitInterval:    250 * time.Millisecond,
					LayeringFactory: autocracy.Factory{},
				},
				dag.Factory{
					EmitInterval:    500 * time.Millisecond,
					LayeringFactory: autocracy.Factory{},
				},
				dag.Factory{
					EmitInterval:    100 * time.Millisecond,
					LayeringFactory: lachesis.Factory{},
				},
				dag.Factory{
					EmitInterval:    250 * time.Millisecond,
					LayeringFactory: lachesis.Factory{},
				},
				dag.Factory{
					EmitInterval:    500 * time.Millisecond,
					LayeringFactory: lachesis.Factory{},
				},
				tendermint.Factory{
					ProposePhaseTimeout:   100 * time.Millisecond,
					PrevotePhaseTimeout:   100 * time.Millisecond,
					PrecommitPhaseTimeout: 100 * time.Millisecond,
					PhaseTimeoutDelta:     10 * time.Millisecond,
				},
				tendermint.Factory{
					ProposePhaseTimeout:   250 * time.Millisecond,
					PrevotePhaseTimeout:   250 * time.Millisecond,
					PrecommitPhaseTimeout: 250 * time.Millisecond,
					PhaseTimeoutDelta:     10 * time.Millisecond,
				},
				tendermint.Factory{
					ProposePhaseTimeout:   500 * time.Millisecond,
					PrevotePhaseTimeout:   500 * time.Millisecond,
					PrecommitPhaseTimeout: 500 * time.Millisecond,
					PhaseTimeoutDelta:     10 * time.Millisecond,
				},
			)),
			Dim(Topology{}, List[p2p.NetworkTopology](
				p2p.NewFullyMeshedTopology(),
			)),
			Dim(NetworkLatencyModel{}, List[p2p.LatencyModel](
				p2p.NewFixedDelayModel().SetBaseDeliveryDelay(10*time.Millisecond),
			)),
		},
	}
}

// --- Study Definition Infrastructure ---

// Property defines a scenario property that can be configured in a study.
type Property[T any] interface {
	Name() string
	Get(*scenario.DemoScenario) T
	Set(*scenario.DemoScenario, T)
}

// Domain defines a value domain for a configurable property in a study.
type Domain[T any] interface {
	All() iter.Seq[T]
}

// Dimension defines a single dimension of variation in a parameter study.
// It combines a property and a domain of values for that property.
type Dimension interface {
	// enumerate generates all scenarios by varying this dimension
	enumerate(*scenario.DemoScenario) iter.Seq[*scenario.DemoScenario]
	// addLabel adds labels for this dimension to the given tracker
	addLabel(tracker.Tracker, scenario.DemoScenario) tracker.Tracker
}

// Dim creates a new dimension for the given property and domain.
func Dim[T any](
	property Property[T],
	domain Domain[T],
) Dimension {
	return &dimension[T]{
		property: property,
		domain:   domain,
	}
}

// dimension is a concrete implementation of the Dimension interface,
// parameterized by the property type. Note that the [Dimension] interface
// is a type-erased interface implemented by this generic type.
type dimension[T any] struct {
	property Property[T]
	domain   Domain[T]
}

func (d *dimension[T]) enumerate(base *scenario.DemoScenario) iter.Seq[*scenario.DemoScenario] {
	return func(yield func(*scenario.DemoScenario) bool) {
		for val := range d.domain.All() {
			d.property.Set(base, val)
			if !yield(base) {
				return
			}
		}
	}
}

func (d *dimension[T]) addLabel(
	tracker tracker.Tracker,
	scenario scenario.DemoScenario,
) tracker.Tracker {
	return tracker.With(
		d.property.Name(),
		d.property.Get(&scenario),
	)
}

type Study struct {
	Dimensions []Dimension
}

func (s Study) All() iter.Seq[scenario.DemoScenario] {
	return s.enumerate(&scenario.DemoScenario{})
}

func (s Study) enumerate(base *scenario.DemoScenario) iter.Seq[scenario.DemoScenario] {
	return func(yield func(scenario.DemoScenario) bool) {
		if len(s.Dimensions) == 0 {
			yield(*base)
			return
		}
		for cur := range s.Dimensions[0].enumerate(base) {
			subStudy := Study{Dimensions: s.Dimensions[1:]}
			for sub := range subStudy.enumerate(cur) {
				if !yield(sub) {
					return
				}
			}
		}
	}
}

func (s Study) AddLabels(
	tracker tracker.Tracker,
	scenario scenario.DemoScenario,
) tracker.Tracker {
	res := tracker
	for _, dim := range s.Dimensions {
		res = dim.addLabel(res, scenario)
	}
	return res
}

// --- Properties ---

type NumNodes struct{}

func (NumNodes) Get(s *scenario.DemoScenario) int {
	return s.NumNodes
}

func (NumNodes) Set(s *scenario.DemoScenario, val int) {
	s.NumNodes = val
}

func (NumNodes) Name() string {
	return "NumNodes"
}

type TxPerSecond struct{}

func (TxPerSecond) Get(s *scenario.DemoScenario) int {
	return s.TxPerSecond
}

func (TxPerSecond) Set(s *scenario.DemoScenario, val int) {
	s.TxPerSecond = val
}

func (TxPerSecond) Name() string {
	return "TxPerSecond"
}

type Broadcast struct{}

func (Broadcast) Get(s *scenario.DemoScenario) broadcast.Protocol {
	return s.Broadcast
}

func (Broadcast) Set(s *scenario.DemoScenario, val broadcast.Protocol) {
	s.Broadcast = val
}

func (Broadcast) Name() string {
	return "Broadcast"
}

type Consensus struct{}

func (Consensus) Get(s *scenario.DemoScenario) consensus.Factory {
	return s.Consensus
}

func (Consensus) Set(s *scenario.DemoScenario, val consensus.Factory) {
	s.Consensus = val
}

func (Consensus) Name() string {
	return "Consensus"
}

type Topology struct{}

func (Topology) Get(s *scenario.DemoScenario) p2p.NetworkTopology {
	return s.Topology
}

func (Topology) Set(s *scenario.DemoScenario, val p2p.NetworkTopology) {
	s.Topology = val
}

func (Topology) Name() string {
	return "Topology"
}

type NetworkLatencyModel struct{}

func (NetworkLatencyModel) Get(s *scenario.DemoScenario) p2p.LatencyModel {
	return s.NetworkLatencyModel
}

func (NetworkLatencyModel) Set(s *scenario.DemoScenario, val p2p.LatencyModel) {
	s.NetworkLatencyModel = val
}

func (NetworkLatencyModel) Name() string {
	return "NetworkLatencyModel"
}

// --- Domains ---

func Single[T any](value T) *list[T] {
	return List(value)
}

func List[T any](values ...T) *list[T] {
	return &list[T]{Values: values}
}

func Range[T constraints.Integer](start, end T) *list[T] {
	return Stride(start, 1, end)
}

func Stride[T constraints.Integer](start, step, end T) *list[T] {
	if step <= 0 {
		panic("step must be positive")
	}
	values := []T{}
	for v := start; v < end; v += step {
		values = append(values, v)
	}
	return List(values...)
}

type list[T any] struct {
	Values []T
}

func (e list[T]) All() iter.Seq[T] {
	return slices.Values(e.Values)
}

func (e list[T]) String() string {
	elements := []string{}
	for _, v := range e.Values {
		elements = append(elements, fmt.Sprintf("%v", v))
	}
	return fmt.Sprintf("[%s]", strings.Join(elements, ", "))
}
