package sim

import (
	"context"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"github.com/urfave/cli/v3"
)

// Run is the main entry point of the Daphne application. It provides some
// general application setup, like the global logger output format, and
// delegates execution to respective sub-commands using the urfave/cli library.
func Run(args []string) error {
	return getApp().Run(context.Background(), args)
}

// getApp builds the application using the urfave/cli library.
func getApp() *cli.Command {
	return &cli.Command{
		Name:  "daphne",
		Usage: "A simulation tool for blockchains",
		Commands: []*cli.Command{
			getRunCommand(),
		},
		Before: setup,
	}
}

// setup initializes global application settings which are relevant for all
// sub-commands of the Daphne application.
func setup(ctxt context.Context, _ *cli.Command) (context.Context, error) {
	// Set a system logger using colored output and a proper time format.
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelInfo,
			TimeFormat: "15:04:05.000",
		}),
	))
	return ctxt, nil
}
