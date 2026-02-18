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

package main

import (
	"fmt"
	"os"

	"github.com/0xsoniclabs/daphne/daphne/sim"
)

// main is the main entry point of the Daphne executable, dispatching commands
// to the appropriate subcommands.
func main() {
	// As the entry point of an executable is difficult to test, the code in
	// this function is deliberately kept minimal, delegating the actual
	// execution to the `sim` package.
	if err := sim.Run(os.Args); err != nil {
		fmt.Printf("Failed to run Daphne: %v\n", err)
		os.Exit(1)
	}
}
