package emitter

import (
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

type PeriodicEmitterFactory struct {
	Interval time.Duration
}

type PeriodicEmitter struct {
	job     *concurrent.Job
	dag     model.Dag
	creator consensus.ValidatorId
}

func (f *PeriodicEmitterFactory) NewEmitter(channel Channel, dag model.Dag, creator consensus.ValidatorId) *PeriodicEmitter {
	emitter := &PeriodicEmitter{
		dag:     dag,
		creator: creator,
	}
	emitter.job = concurrent.StartPeriodicJob(f.Interval, func(t time.Time) {
		channel.Emit(emitter.dag.GetHeads())
	})

	return emitter
}

func (e *PeriodicEmitter) OnDagChange() {
	// No-op
}

func (e *PeriodicEmitter) Stop() {
	e.job.Stop()
}
