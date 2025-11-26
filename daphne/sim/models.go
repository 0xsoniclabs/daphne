package sim

import (
	"time"

	"github.com/0xsoniclabs/daphne/daphne/state"
	"github.com/0xsoniclabs/daphne/daphne/utils"
)

// getDefaultStateProcessingLatencyModel returns a sampled processing delay
// model with log-normal distributions fitted to real-world transaction latency
// data. The model has been fitted to match the observed percentiles of
// transaction and block finalization latencies on the Sonic main chain for the
// first 5 million blocks.
func getDefaultStateProcessingLatencyModel() *state.SampledProcessingDelayModel {
	return _defaultStateProcessingLatencyModel
}

// _defaultStateProcessingLatencyModel is the canonical instance of the latency
// model required to enable comparability in tests.
var _defaultStateProcessingLatencyModel = func() *state.SampledProcessingDelayModel {
	model := state.NewSampledProcessingDelayModel()
	model.SetBaseTransactionDistribution(
		utils.NewLogNormalDistribution(11.484, 1.396, time.Nanosecond, nil),
	)
	model.SetBaseBlockFinalizationDistribution(
		utils.NewLogNormalDistribution(4.054, 1.367, time.Nanosecond, nil),
	)
	return model
}()
