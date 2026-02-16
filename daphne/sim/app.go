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
			getEvalCommand(),
			getStudyCommand(),
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
