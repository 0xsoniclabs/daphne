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

package concurrent

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJob_Stop_CanBeCalledOnZeroJob(t *testing.T) {
	job := Job{}
	job.Stop()
}

func TestJob_Stop_CanBeCalledMoreThanOnce(t *testing.T) {
	job := StartPeriodicJob(time.Second, func(time.Time) {
		t.Errorf("unexpected call to task")
	})
	job.Stop()
	job.Stop() // < no panic for closing a closed channel
}

func TestJob_Stop_IsSignaledToJobForGracefulShutdown(t *testing.T) {
	job := StartJob(func(stop <-chan struct{}) {
		<-stop // would block forever if not signaled
	})
	job.Stop() // < if it completes, the job has finished
}

func TestJob_Stop_BlocksUntilJobIsFinished(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		job := StartJob(func(stop <-chan struct{}) {
			time.Sleep(time.Second)
		})
		job.Stop()
		require.Equal(t, time.Since(start), time.Second)
	})
}

func TestJob_CanStartAndStopPeriodicJob(t *testing.T) {
	period := 1 * time.Second
	synctest.Test(t, func(t *testing.T) {
		counter := 0
		done := make(chan struct{})
		start := time.Now()
		last := start
		job := StartPeriodicJob(period, func(time time.Time) {
			delay := time.Sub(last)
			if last.Equal(start) {
				require.LessOrEqual(
					t, delay, period,
					"initial delay was too high",
				)
			} else {
				require.Equal(
					t, delay, period,
					"unexpected interval between periodic calls",
				)
			}
			last = time
			counter++
			if counter == 3 {
				close(done)
			}
		})

		select {
		case <-time.After(5 * time.Second):
			t.Error("failed to run jobs within expected time")
		case <-done:
		}
		job.Stop()
	})
}

func TestJob_CanStopPeriodicJobBeforeFirstTrigger(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		period := 1 * time.Second
		job := StartPeriodicJob(period, func(time.Time) {
			t.Errorf("unexpected call to task")
		})
		job.Stop()
	})
}
