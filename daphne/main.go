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
