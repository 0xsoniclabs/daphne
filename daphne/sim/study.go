package sim

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/central"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/sim/scenario"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/0xsoniclabs/daphne/daphne/tracker/mark"
	"github.com/urfave/cli/v3"
	"golang.org/x/exp/constraints"
)

// getStudyCommand assembles the "study" sub-command of the Daphne application
// intended to run a range of simulation scenarios defined by a parameter study.
func getStudyCommand() *cli.Command {
	return &cli.Command{
		Name:   "study",
		Usage:  "Runs a parameter study over multiple simulation scenarios",
		Action: studyAction,
		Flags: []cli.Flag{
			simTimeFlag,
			outputFileFlag,
		},
	}
}

func studyAction(ctx context.Context, c *cli.Command) error {
	outputFile := c.String(outputFileFlag.Name)
	slog.Info("Results will be saved to", "file", outputFile)
	root, err := tracker.New(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create root tracker: %w", err)
	}

	study := defaultStudy()
	all := slices.Collect(study.All())
	slog.Info("Running study", "num_scenarios", len(all))
	for i, scenario := range all {
		slog.Info("Running scenario", "index", i+1, "of", len(all))

		tracker := root.With("sid", i)
		study.AddLabels(tracker, scenario).Track(mark.StudyStarted)

		if err := runScenarioWithTracker(c, &scenario, tracker); err != nil {
			return fmt.Errorf("failed scenario %d/%d: %w", i+1, len(all), err)
		}
	}

	slog.Info("Flushing results to disk")
	root.Stop()
	return nil
}

func defaultStudy() Study {

	// TODO: find a way to change the default of those factories
	gossip := broadcast.Factories{}
	//forwarding := broadcast.Factories{}

	return Study{
		Dimensions: []Dimension{
			Dim(NumNodes{}, Range(1, 20)), //List(5, 10, 15, 20)), //Stride(5, 5, 51)),
			Dim(TxPerSecond{}, List(5, 10, 20)),
			Dim(Duration{}, List(10*time.Second)),
			Dim(Broadcaster{}, List(gossip)),
			Dim(Consensus{}, List[consensus.Factory](
				central.Factory{
					Leader: p2p.PeerId("N-001"),
				},
			)),
			Dim(Topology{}, List[p2p.NetworkTopology](
				p2p.NewFullyMeshedTopology(),
			)),
		},
	}
}

type Property[T any] interface {
	Get(*scenario.DemoScenario) T
	Set(*scenario.DemoScenario, T)
	Name() string
}

type Domain[T any] interface {
	All() iter.Seq[T]
}

type Dimension interface {
	enumerate(*scenario.DemoScenario) iter.Seq[*scenario.DemoScenario]
	addLabel(tracker.Tracker, scenario.DemoScenario) tracker.Tracker
}

func Dim[T any](
	property Property[T],
	domain Domain[T],
) Dimension {
	return &dimension[T]{
		property: property,
		domain:   domain,
	}
}

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

type Duration struct{}

func (Duration) Get(s *scenario.DemoScenario) time.Duration {
	return s.Duration
}

func (Duration) Set(s *scenario.DemoScenario, val time.Duration) {
	s.Duration = val
}

func (Duration) Name() string {
	return "Duration"
}

type Broadcaster struct{}

func (Broadcaster) Get(s *scenario.DemoScenario) broadcast.Factories {
	return s.Broadcaster
}

func (Broadcaster) Set(s *scenario.DemoScenario, val broadcast.Factories) {
	s.Broadcaster = val
}

func (Broadcaster) Name() string {
	return "Broadcaster"
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
