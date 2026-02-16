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

package emitter

import (
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
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
func (f PeriodicEmitterFactory) NewEmitter(channel Channel, dag model.Dag, creator consensus.ValidatorId, _ layering.Layering) Emitter {
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
