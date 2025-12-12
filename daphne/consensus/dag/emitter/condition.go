package emitter

import (
	"maps"
	"slices"
	"sync/atomic"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

//go:generate mockgen -source condition.go -destination=condition_mock.go -package=emitter

// Condition defines the interface for emission conditions.
type Condition interface {
	// Reset resets the internal state of the condition.
	// It should be called whenever a new emission cycle starts.
	Reset(emitter *Emitter, dagSnapshot map[consensus.ValidatorId]*model.Event)
	// Evaluate evaluates the condition against the current state of the emitter.
	Evaluate(emitter *Emitter) bool
	Stop()
}

// --- Common Condition implementations ---

// --- Timeout Condition ---

type timeoutCondition struct {
	duration time.Duration

	job            *concurrent.Job
	timeoutOccured atomic.Bool
}

// NewTimeoutCondition creates a new timeout-based emission condition.
// An emission is triggered if the specified duration has elapsed since the last emission.
func NewTimeoutCondition(duration time.Duration) *timeoutCondition {
	return &timeoutCondition{
		duration: duration,
	}
}

func (c *timeoutCondition) Reset(emitter *Emitter, _ map[consensus.ValidatorId]*model.Event) {
	if c.job != nil {
		c.job.Stop()
	}

	c.timeoutOccured.Store(false)
	c.job = concurrent.StartJob(func(stop <-chan struct{}) {
		select {
		case <-time.After(c.duration):
			c.timeoutOccured.Store(true)

			go func() {
				emitter.AttemptEmission()
			}()
			return
		case <-stop:
			return
		}
	})
}

func (c *timeoutCondition) Evaluate(*Emitter) bool {
	return c.timeoutOccured.Load()
}

func (c *timeoutCondition) Stop() {
	if c.job != nil {
		c.job.Stop()
	}
}

// --- Observes Latest Emission Condition ---

type observesLatestEmissionCondition struct{}

// NewObservesLatestEmissionCondition creates a new condition that triggers
// an emission when the emitter has observed the latest emission it made.
// This prevents double-signing.
func NewObservesLatestEmissionCondition() *observesLatestEmissionCondition {
	return &observesLatestEmissionCondition{}
}

func (*observesLatestEmissionCondition) Reset(*Emitter, map[consensus.ValidatorId]*model.Event) {}

func (*observesLatestEmissionCondition) Evaluate(emitter *Emitter) bool {
	dagHeads := emitter.getDag().GetHeads()
	lastSeenEvent, emittedAtLeastOnce := dagHeads[emitter.getCreator()]

	latestObservedSeq := uint32(0)
	if emittedAtLeastOnce {
		latestObservedSeq = lastSeenEvent.Seq()
	}

	return latestObservedSeq >= emitter.getLastEmittedSeq()
}

func (*observesLatestEmissionCondition) Stop() {}

// --- Observes New Parents Condition ---

type observesNewParentsCondition struct {
	numParents         int
	lastEmittedParents map[consensus.ValidatorId]*model.Event
}

// NewObservesNewParentsCondition creates a new condition that triggers
// an emission when the emitter has observed at least numParents new parents
// since its last emission.
func NewObservesNewParentsCondition(numParents int) *observesNewParentsCondition {
	return &observesNewParentsCondition{numParents: numParents}
}

func (o *observesNewParentsCondition) Reset(e *Emitter, dagSnapshot map[consensus.ValidatorId]*model.Event) {
	o.lastEmittedParents = dagSnapshot
}

func (o *observesNewParentsCondition) Evaluate(emitter *Emitter) bool {
	dagHeads := emitter.getDag().GetHeads()
	_, emittedAtLeastOnce := dagHeads[emitter.getCreator()]
	if !emittedAtLeastOnce {
		return true
	}

	prev := sets.New(slices.Collect(maps.Values(o.lastEmittedParents))...)
	new := sets.New(slices.Collect(maps.Values(dagHeads))...)
	diff := sets.Difference(new, prev)

	return diff.Size() >= o.numParents
}

func (*observesNewParentsCondition) Stop() {}

// --- Composite Conditions ---

type orCondition struct {
	conds []Condition
}

// NewOrCondition creates a new condition that triggers an emission
// if any of the provided conditions evaluate to true.
func NewOrCondition(conds ...Condition) *orCondition {
	return &orCondition{conds: conds}
}

func (o *orCondition) Reset(emitter *Emitter, dagSnapshot map[consensus.ValidatorId]*model.Event) {
	for _, cond := range o.conds {
		cond.Reset(emitter, dagSnapshot)
	}
}

func (o *orCondition) Evaluate(emitter *Emitter) bool {
	for _, cond := range o.conds {
		if cond.Evaluate(emitter) {
			return true
		}
	}
	return false
}

func (o *orCondition) Stop() {
	for _, cond := range o.conds {
		cond.Stop()
	}
}

// NewAndCondition creates a new condition that triggers an emission
// only if all of the provided conditions evaluate to true.
func NewAndCondition(conds ...Condition) *andCondition {
	return &andCondition{conds: conds}
}

type andCondition struct {
	conds []Condition
}

func (a *andCondition) Reset(emitter *Emitter, dagSnapshot map[consensus.ValidatorId]*model.Event) {
	for _, cond := range a.conds {
		cond.Reset(emitter, dagSnapshot)
	}
}

func (a *andCondition) Evaluate(emitter *Emitter) bool {
	for _, cond := range a.conds {
		if !cond.Evaluate(emitter) {
			return false
		}
	}
	return true
}

func (a *andCondition) Stop() {
	for _, cond := range a.conds {
		cond.Stop()
	}
}

// --- False Condition ---

type FalseCondition struct{}

func NewFalseCondition() Condition {
	return &FalseCondition{}
}

func (*FalseCondition) Reset(*Emitter, map[consensus.ValidatorId]*model.Event) {}

func (*FalseCondition) Stop() {}
func (*FalseCondition) Evaluate(*Emitter) bool {
	return false
}
