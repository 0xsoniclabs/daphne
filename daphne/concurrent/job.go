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
	"math/rand/v2"
	"time"
)

// Job is a handle to an operation running in another goroutine that can be
// stopped on demand. It is intended to simplify the creation and management of
// background operations that need to be stopped at some point.
//
// A typical use case would look like this:
//
//	job := StartPeriodicJob(time.Second, func(t time.Time) {
//	    // Do some work...
//	})
//
// where it can be eventually stopped using
//
//	job.Stop()
//
// Operations on a Job are not thread-safe.
type Job struct {
	stop chan<- struct{}
	done <-chan struct{}
}

// Stop signals the job to stop and waits for it to finish. A second call to
// Stop or attempting to stop a non-running job has no effect.
func (j *Job) Stop() {
	if j.stop == nil {
		return
	}
	close(j.stop)
	<-j.done
	j.stop = nil
	j.done = nil
}

// StartJob creates a new goroutine running the given task in the background
// until it either completes or Stop is called on the returned Job. The task
// should use the provided channel to detect stop requests.
func StartJob(task func(<-chan struct{})) *Job {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		task(stop)
	}()
	return &Job{
		stop: stop,
		done: done,
	}
}

// StartPeriodicJob creates a new goroutine running the given task at regular
// intervals until Stop is called on the returned Job. The first execution will
// happen after a random interval between 0 and the given period to avoid
// clustering of periodic jobs with the same period in the system.
func StartPeriodicJob(
	period time.Duration,
	task func(time.Time),
) *Job {
	return StartJob(func(stop <-chan struct{}) {
		// Do a random offset at the beginning to avoid clustering of
		// periodic jobs with the same period.
		firstDelay := time.Duration(float64(period) * rand.Float64())
		select {
		case time := <-time.After(firstDelay):
			task(time)
		case <-stop:
			return
		}
		// After the first run, switch to a ticker.
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			select {
			case time := <-ticker.C:
				task(time)
			case <-stop:
				return
			}
		}
	})
}
