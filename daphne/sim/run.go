package sim

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/sim/scenario"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/urfave/cli/v3"
)

var (
	outputFileFlag = &cli.StringFlag{
		Name:    "output-file",
		Aliases: []string{"o"},
		Usage:   "Path to the output file for the simulation results",
		Value:   "output.csv",
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
		},
	}
}

func runAction(ctx context.Context, c *cli.Command) error {
	scenario := loadScenario(c)
	return runScenario(c, scenario)
}

func loadScenario(c *cli.Command) scenario.Scenario {
	// For now, this command runs a simple place-holder scenario. In the future,
	// this scenario should be replaced by a scripted setup.
	return &scenario.DemoScenario{
		NumNodes:    c.Int(numNodesFlag.Name),
		TxPerSecond: c.Int(txPerSecondFlag.Name),
		Duration:    c.Duration(durationFlag.Name),
	}
}

// runScenario executes the given scenario using the user-selected execution
// mode (real-time or sim-time) and handles any errors that occur.
func runScenario(
	c *cli.Command,
	scenario scenario.Scenario,
) error {
	outputFile := c.String(outputFileFlag.Name)
	run := runRealTime
	mode := "real-time"
	if c.Bool(simTimeFlag.Name) {
		mode = "simulated-time"
		run = runSimTime
	}

	slog.Info("Running scenario", "mode", mode, "outputFile", outputFile)

	root := tracker.New()
	err := run(root, scenario)
	if err != nil {
		slog.Error("Failed to run simulation", "error", err)
		return err
	}

	// Step 3: collecting and exporting data
	slog.Info("Collecting and exporting data")
	data := root.GetAll()
	if err := exportData(data, outputFile); err != nil {
		slog.Error("Failed to export collected data", "error", err)
		return err
	}

	slog.Info("Exported collected data", "numRecords", len(data), "file", outputFile)
	return nil
}

// exportData is a utility function handling the export of collected tracker
// data to a file.
func exportData(
	data []tracker.Entry,
	outputFile string,
) (err error) {
	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputFile, err)
	}
	defer func() {
		err = errors.Join(err, out.Close())
	}()
	return tracker.ExportAsCSV(data, out)
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
