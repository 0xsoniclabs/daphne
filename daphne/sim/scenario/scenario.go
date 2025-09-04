package scenario

import "github.com/0xsoniclabs/daphne/daphne/tracker"

//go:generate mockgen -source scenario.go -destination=scenario_mock.go -package=scenario

// Scenario is a simulation scenario running within the Daphne evaluation tool.
type Scenario interface {
	Run(Logger, tracker.Tracker) error
}
