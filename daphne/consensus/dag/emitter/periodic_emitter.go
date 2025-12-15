package emitter

import (
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

// PeriodicEmitterFactory is a factory for creating [PeriodicEmitter] instances.
// PeriodicEmitter emits new events at fixed time intervals, regardless of DAG changes.
type PeriodicEmitterFactory struct {
	// Interval defines the time interval between successive emissions.
	Interval time.Duration
}

func (f PeriodicEmitterFactory) String() string {
	return "periodic_" + f.Interval.String()
}

type PeriodicEmitter struct {
	job *concurrent.Job
}

// NewEmitter creates a new [PeriodicEmitter] that emits events at fixed intervals.
// The first emission interval starts immediately upon creation.
func (f PeriodicEmitterFactory) NewEmitter(channel Channel, dag model.Dag, creator consensus.ValidatorId) Emitter {
	return &PeriodicEmitter{
		job: concurrent.StartPeriodicJob(f.Interval, func(t time.Time) {
			channel.Emit(dag.GetHeads())
		}),
	}
}

func (e *PeriodicEmitter) OnChange() {}

func (e *PeriodicEmitter) Stop() {
	e.job.Stop()
}
