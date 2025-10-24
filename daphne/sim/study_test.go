package sim

import (
	"slices"
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/sim/scenario"
	"github.com/stretchr/testify/require"
)

func TestDefaultStudy_HasLessThan100Scenarios(t *testing.T) {
	// This is not a hard test, just a sanity check to ensure that the
	// default study does not cover too many scenarios.
	study := defaultStudy()
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
	study := Study{
		Dimensions: []Dimension{
			Dim(NumNodes{}, List(1, 2, 4, 8, 16)),
			Dim(TxPerSecond{}, List(10, 100)),
			Dim(Duration{}, List(10*time.Second, 1*time.Minute)),
		},
	}
	all := slices.Collect(study.All())
	require.Len(t, all, 5*2*2)

	for _, scenario := range all {
		require.Contains(t, []int{1, 2, 4, 8, 16}, scenario.NumNodes)
		require.Contains(t, []int{10, 100}, scenario.TxPerSecond)
		require.Contains(t, []time.Duration{10 * time.Second, 1 * time.Minute}, scenario.Duration)
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
