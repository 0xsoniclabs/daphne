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

// Copyright 2026 Sonic Labs
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
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/sim/scenario"
	"github.com/0xsoniclabs/daphne/daphne/tracker"
	"github.com/stretchr/testify/require"
)

func TestStudyAction_CanBeRun(t *testing.T) {
	// This is a top-level smoke test to ensure that the study action can be run
	// without errors.
	studies := []string{
		"broadcast",
		"consensus",
		"load",
		"topology",
	}

	for _, studyName := range studies {
		t.Run(studyName, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "output.parquet")
			command := getStudyCommand()
			require.NotNil(t, command)
			require.NoError(t, command.Run(t.Context(), []string{
				"study", studyName,
				"-o", output,
				"-d", "1ms",
			}))
			require.FileExists(t, output)
		})
	}
}

func TestStudyAction_DurationMustBePositive(t *testing.T) {
	durations := []string{"0s", "-1s", "-100ms"}
	for _, dur := range durations {
		command := getStudyCommand()
		require.NotNil(t, command)
		err := command.Run(t.Context(), []string{
			"study", "broadcast",
			"-o", filepath.Join(t.TempDir(), "output.parquet"),
			"-d", dur,
		})
		require.ErrorContains(t, err, "duration must be positive", "duration: %s", dur)
	}
}

func TestStudyAction_InvalidOutputLocation_ReportsOutputError(t *testing.T) {
	err := _studyAction(t.Context(), StudyConfig{
		RunConfig: RunConfig{
			outputFile: t.TempDir(), // < can not write to a directory
		},
	}, getLoadStudy(), nil)
	require.ErrorContains(t, err, "is a directory")
}

func TestStudyAction_ContextCancellation_StopsExecution(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(100*time.Millisecond))
	defer cancel()

	cancelled := false
	run := func(scenario scenario.Scenario, tracker tracker.Tracker) error {
		require.False(t, cancelled, "runner called after cancellation")
		cancel() // < cancelled after the first scenario
		cancelled = true
		return nil
	}

	err := _studyAction(ctx, StudyConfig{
		RunConfig: RunConfig{
			outputFile: filepath.Join(t.TempDir(), "output.parquet"),
		},
		duration: 500 * time.Millisecond,
	}, getLoadStudy(), run)
	require.ErrorIs(t, err, context.Canceled)
}

func TestStudyAction_FailingScenario_EndsStudyAndReportsError(t *testing.T) {
	issue := fmt.Errorf("scenario failure")
	run := func(scenario scenario.Scenario, tracker tracker.Tracker) error {
		return issue
	}

	err := _studyAction(t.Context(), StudyConfig{
		RunConfig: RunConfig{
			outputFile: filepath.Join(t.TempDir(), "output.parquet"),
		},
		duration: 500 * time.Millisecond,
	}, getLoadStudy(), run)
	require.ErrorIs(t, err, issue)
}

func TestGetLoadStudy_HasLessThan100Scenarios(t *testing.T) {
	// This is not a hard test, just a sanity check to ensure that the
	// default study does not cover too many scenarios.
	study := getLoadStudy()
	all := slices.Collect(study.All())
	require.Less(t, len(all), 100)
}

func TestStudy_EmptyStudyYieldsDefaultScenario(t *testing.T) {
	study := Study{}
	all := slices.Collect(study.All())
	require.Len(t, all, 1)
	require.Equal(t, scenario.DemoScenario{}, all[0])
}

func TestStudy_MultipleDomainsProduceCartesianProduct(t *testing.T) {

	topologyA := p2p.FullyMeshedTopologyFactory{}
	topologyB := p2p.LineTopologyFactory{}

	study := Study{
		Dimensions: []Dimension{
			Dim(NumValidators{}, List(1, 2, 4, 8, 16)),
			Dim(TxPerSecond{}, List(10, 100)),
			Dim(Topology{}, List[p2p.TopologyFactory](
				topologyA,
				topologyB,
			)),
		},
	}
	all := slices.Collect(study.All())
	require.Len(t, all, 5*2*2)

	for _, scenario := range all {
		require.Contains(t, []int{1, 2, 4, 8, 16}, scenario.NumValidators)
		require.Contains(t, []int{10, 100}, scenario.TxPerSecond)
		require.Contains(t, []p2p.TopologyFactory{topologyA, topologyB}, scenario.Topology)
	}
}

func TestStudy_enumeration_Abort_DoesNotPanic(t *testing.T) {
	for range getLoadStudy().All() {
		break // abort immediately
	}
}

func TestDimension_enumeration_Abort_DoesNotPanic(t *testing.T) {
	dim := Dim(NumValidators{}, Range(1, 1000000))
	scenario := &scenario.DemoScenario{}
	for range dim.enumerate(scenario) {
		break // abort immediately
	}
}

func TestSingle_ContainsSingleElement(t *testing.T) {
	require.Equal(t, "[12]", Single(12).String())
	require.Equal(t, "[42]", Single(42).String())
}

func TestList_ContainsListedElement(t *testing.T) {
	require.Equal(t, "[]", List[int]().String())
	require.Equal(t, "[1, 7]", List(1, 7).String())
	require.Equal(t, "[1, 8, 5]", List(1, 8, 5).String())
}

func TestRange_ContainsElementsInRange(t *testing.T) {
	require.Equal(t, "[1, 2, 3]", Range(1, 4).String())
	require.Equal(t, "[7, 8]", Range(7, 9).String())
	require.Equal(t, "[]", Range(5, 5).String())
	require.Equal(t, "[]", Range(8, 5).String())
}

func TestStride_ContainsElementsInRange(t *testing.T) {
	require.Equal(t, "[1, 3, 5]", Stride(1, 2, 6).String())
	require.Equal(t, "[0, 5, 10]", Stride(0, 5, 12).String())
	require.Equal(t, "[2, 4, 6, 8]", Stride(2, 2, 10).String())
	require.Equal(t, "[]", Stride(5, 1, 5).String())
	require.Equal(t, "[]", Stride(10, 1, 5).String())
}

func TestStride_ZeroStepPanics(t *testing.T) {
	require.Panics(t, func() {
		Stride(1, 0, 10)
	})
}

func TestStride_NegativeStepPanics(t *testing.T) {
	require.Panics(t, func() {
		Stride(10, -2, 1)
	})
}

func TestGetTopologyStudy_HasLessThan100Scenarios(t *testing.T) {
	// This is just a sanity check to ensure that the
	// topology study does not cover too many scenarios.
	study := getTopologyStudy()
	all := slices.Collect(study.All())
	require.Less(t, len(all), 100)
}

func TestGetTopologyStudy_ContainsExpectedTopologies(t *testing.T) {
	study := getTopologyStudy()
	all := slices.Collect(study.All())

	// The topology study should contain scenarios with different topologies
	require.NotEmpty(t, all)

	// Collect all unique topology string representations
	topologyStrings := make(map[string]bool)
	for _, scenario := range all {
		if scenario.Topology != nil {
			// TopologyFactory implements fmt.Stringer
			topologyStrings[scenario.Topology.String()] = true
		}
	}

	// Should have multiple different topology types
	require.Contains(t, topologyStrings, "fully-meshed")
	require.Contains(t, topologyStrings, "line")
	require.Contains(t, topologyStrings, "ring")
	require.Contains(t, topologyStrings, "star")
	require.Contains(t, topologyStrings, "random-3-seed42")
	require.Contains(t, topologyStrings, "random-5-seed42")
	require.Contains(t, topologyStrings, "random-10-seed42")
	require.Contains(t, topologyStrings, "two-layer-seed42")
}
