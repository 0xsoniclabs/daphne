package emitter

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

// PartialSynchronyEmitterFactory is a factory for creating [PartialSynchronyEmitter] instances.
// PartialSynchronyEmitter emits new events based on DAG changes and a partial synchrony timeout
// that assumes eventual message delivery within a known bound which is guaranteed after network
// reaches Global Stabilization Time (GST).
//
// It is designed to operate on structured DAGs where events are organized into rounds by the layering.
//
// Each round has a designated primary event (candidate) determined by the layering strategy.
// The goal of the underlying consensus is to ensure that these primary events are finalized in order.
// To that end the emitter enforces specific emission conditions based on the structure of the DAG,
// the placement of primaries, and their relations:
//
//  1. Condition - requires observing the latest emission from the emitter itself. This prevents
//     double signing.
//
//  2. Condition - requires an observation of quorum of events from the round previous to the emission
//     target round.
//
//  3. Condition:
//     - Requires an observation of the primary for the round before the target round.
//     AND
//     - Requires an observation of a quorum of events from round before the target round that
//     reach the primary for the round two rounds before the target.
//     -------------------------------------------------------------------------------------------
//     OR
//     -------------------------------------------------------------------------------------------
//     - Requires a TimeoutDuration of time to have passed since the last emission.
//
//     This condition ensures progress even in cases where primaries are not observed due to network
//     delays or faults. The TimeoutDuration should be set according to the expected network conditions,
//     such that all deliveries are happening well within the bounds of it, during periods of GST.
type PartialSynchronyEmitterFactory struct {
	Committee       *consensus.Committee
	TimeoutDuration time.Duration
}

type PartialSynchronyEmitter struct {
	dag       model.Dag
	committee *consensus.Committee
	creator   consensus.ValidatorId
	layering  layering.Layering

	latestEmittedRound uint32

	channel Channel

	timeoutJob      *concurrent.Job
	timeoutDuration time.Duration
	timeoutOccurred atomic.Bool

	stateMutex sync.Mutex
}

func (f PartialSynchronyEmitterFactory) NewEmitter(channel Channel, dag model.Dag, creator consensus.ValidatorId, layering layering.Layering) Emitter {
	return newPartialSynchronyEmitter(dag, f.Committee, creator, f.TimeoutDuration, channel, layering)
}

func newPartialSynchronyEmitter(
	dag model.Dag,
	committee *consensus.Committee,
	creator consensus.ValidatorId,
	timeout time.Duration,
	channel Channel,
	layering layering.Layering,
) *PartialSynchronyEmitter {
	e := &PartialSynchronyEmitter{
		dag:             dag,
		committee:       committee,
		creator:         creator,
		timeoutDuration: timeout,
		channel:         channel,
		layering:        layering,
	}
	e.startTimeoutTimer()
	return e
}

func (f PartialSynchronyEmitterFactory) String() string {
	return "psynchrony_" + f.TimeoutDuration.String()
}

func (e *PartialSynchronyEmitter) OnChange() {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	if e.channel == nil {
		return
	}

	dagHeads := e.dag.GetHeads()
	roundToEmit := e.roundToEmit(dagHeads)
	if e.shouldEmit(dagHeads, roundToEmit) {
		e.channel.Emit(e.selectParents(dagHeads, roundToEmit))

		e.latestEmittedRound = roundToEmit
		e.timeoutOccurred.Store(false)
		e.startTimeoutTimer()
	}
}

func (e *PartialSynchronyEmitter) Stop() {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	e.timeoutJob.Stop()
	e.channel = nil
}

// startTimeoutTimer starts or restarts the timeout timer,
// stopping any existing timer job if it exists.
func (e *PartialSynchronyEmitter) startTimeoutTimer() {
	if e.timeoutJob != nil {
		e.timeoutJob.Stop()
	}
	e.timeoutJob = concurrent.StartJob(func(stop <-chan struct{}) {
		select {
		case <-time.After(e.timeoutDuration):
			e.timeoutOccurred.Store(true)
			go e.OnChange()
		case <-stop:
		}
	})
}

// selectParents selects parents for the new event to be emitted for the given round.
// It selects the latest event from each validator that is at most at round (roundToEmit - 1).
// For validators whose latest event is lower than (roundToEmit-1), it selects their latest event as is.
func (e *PartialSynchronyEmitter) selectParents(dagHeads map[consensus.ValidatorId]*model.Event, roundToEmit uint32) map[consensus.ValidatorId]*model.Event {
	parents := map[consensus.ValidatorId]*model.Event{}
	for validatorId, head := range dagHeads {
		for e.layering.GetRound(head) > roundToEmit-1 {
			head = head.SelfParent()
		}
		parents[validatorId] = head
	}
	return parents
}

func (e *PartialSynchronyEmitter) shouldEmit(dagHeads map[consensus.ValidatorId]*model.Event, roundToEmit uint32) bool {
	// Always emit for the genesis round.
	if e.latestEmittedRound == 0 {
		return true
	}

	if !e.observesLatestEmisstion(dagHeads) {
		return false
	}

	if !e.observesQuorumOfRound(dagHeads, roundToEmit-1) {
		return false
	}

	return e.timeoutOccurred.Load() || e.supportsThePrimaries(dagHeads, roundToEmit)
}

// roundToEmit determines the next round to emit for. It starts from the round
// after the latest emitted round and increments until it finds a round
// that has not yet passed the "too late to emit" condition. This condition
// prevents "futile" emission attempts in which the round has already been
// populated with a quorum of votes, meaning that other validators are likely*
// already preparing their next emissions for subsequent rounds, and won't
// be able to include our emission directly in those.
func (e *PartialSynchronyEmitter) roundToEmit(dagHeads map[consensus.ValidatorId]*model.Event) uint32 {
	round := e.latestEmittedRound + 1

	tooLateToEmit := func(targetRound uint32) bool {
		counter := consensus.NewVoteCounter(e.committee)
		roundMembers := e.getRoundMembers(dagHeads, targetRound)
		for member := range roundMembers.All() {
			counter.Vote(member.Creator())
		}

		return counter.IsQuorumReached()
	}

	for tooLateToEmit(round) {
		round++
	}

	return round
}

func (e *PartialSynchronyEmitter) observesLatestEmisstion(dagHeads map[consensus.ValidatorId]*model.Event) bool {
	lastObservedEvent, observesGenesisEmission := dagHeads[e.creator]

	lastObservedEventSeq := uint32(0)
	if observesGenesisEmission {
		lastObservedEventSeq = lastObservedEvent.Seq()
	}

	return lastObservedEventSeq >= e.latestEmittedRound
}

func (e *PartialSynchronyEmitter) observesQuorumOfRound(dagHeads map[consensus.ValidatorId]*model.Event, round uint32) bool {
	roundMembers := e.getRoundMembers(dagHeads, round)

	voteCounter := consensus.NewVoteCounter(e.committee)
	for parent := range roundMembers.All() {
		voteCounter.Vote(parent.Creator())
	}

	return voteCounter.IsQuorumReached()
}

// supportsThePrimaries checks whether the conditions related to primaries are met (Condition 3)
// It divides the condition into veryifying the support for the later primary (targetRound - 1)
// and the earlier primary (targetRound - 2).
func (e *PartialSynchronyEmitter) supportsThePrimaries(dagHeads map[consensus.ValidatorId]*model.Event, targetRound uint32) bool {
	if e.latestEmittedRound == 0 {
		return true
	}

	// The algorithm is relying on a guaranteed provided by layering,
	// that there is at most one primary (candidate) per round.

	laterRoundMembers := e.getRoundMembers(dagHeads, targetRound-1)
	laterPrimarySet := sets.Filter(laterRoundMembers, func(event *model.Event) bool {
		return e.layering.IsCandidate(event)
	})
	// If no primary is observed for the later round, the condition can fail early.
	if laterPrimarySet.IsEmpty() {
		return false
	}
	// If we are only at round 1, observing only the later primary is sufficient.
	if e.latestEmittedRound == 1 {
		return true
	}

	earlierPrimarySet := sets.Filter(e.getRoundMembers(dagHeads, targetRound-2), func(event *model.Event) bool {
		return e.layering.IsCandidate(event)
	})
	if earlierPrimarySet.IsEmpty() {
		return false
	}

	earlierPrimary := earlierPrimarySet.ToSlice()[0]
	voteCounter := consensus.NewVoteCounter(e.committee)
	for voter := range laterRoundMembers.All() {
		if e.dag.Reaches(voter, earlierPrimary) {
			voteCounter.Vote(voter.Creator())
		}
	}
	// If the later primary condition has passed, and there is a quorum of
	// votes from the later round that reach the earlier primary, the condition passes.
	return voteCounter.IsQuorumReached()
}

func (e *PartialSynchronyEmitter) getRoundMembers(dagHeads map[consensus.ValidatorId]*model.Event, targetRound uint32) sets.Set[*model.Event] {
	members := sets.New[*model.Event]()

	for _, head := range dagHeads {
		head.TraverseClosure(model.WrapEventVisitor(func(event *model.Event) model.VisitResult {
			eventRound := e.layering.GetRound(event)

			if members.Contains(event) || eventRound < targetRound {
				return model.Visit_Prune
			}

			if eventRound == targetRound {
				members.Add(event)
			}

			return model.Visit_Descent
		}))
	}

	return members
}
