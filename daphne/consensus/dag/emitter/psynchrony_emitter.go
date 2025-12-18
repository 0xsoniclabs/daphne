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
// that assumes eventual message delivery within a known bound after Global Stabilization Time (GST).
// It requires to observe quorum of emissions from the last round and certain leader support depending
// on the round within the current wave:
// - On round 0, it fulfills the conditions immediately.
// - On round 1, it emits if it observes the primary leader's event.
// - On round 2, it emits if it observes a quorum of events that reach the primary leader's event.
type PartialSynchronyEmitterFactory struct {
	Committee *consensus.Committee
	Timeout   time.Duration
	// TODO: as a PoC, the Leader is fixed for the whole duration. This should be replaced
	// with a proper Leader choosing mechanism.
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
	return newPartialSynchronyEmitter(dag, f.Committee, creator, f.Timeout, channel, layering)
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
	return "psynchrony_" + f.Timeout.String()
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

	// If any of the mandatory conditions is not met, reject.
	if !e.observesLatestEmisstion(dagHeads) {
		return false
	}

	if !e.observesQuorumOfRound(dagHeads, roundToEmit-1) {
		return false
	}

	return e.timeoutOccurred.Load() || e.supportsThePrimaries(dagHeads, roundToEmit)
}

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

func (e *PartialSynchronyEmitter) supportsThePrimaries(dagHeads map[consensus.ValidatorId]*model.Event, targetRound uint32) bool {
	if e.latestEmittedRound == 0 {
		return true
	}

	laterRoundMembers := e.getRoundMembers(dagHeads, targetRound-1)
	// Look a round back to find the later primary.
	laterPrimarySet := sets.Filter(laterRoundMembers, func(event *model.Event) bool {
		return e.layering.IsCandidate(event)
	})
	laterPrimaryObserved := !laterPrimarySet.IsEmpty()
	if !laterPrimaryObserved {
		return false
	}

	if e.latestEmittedRound == 1 {
		return laterPrimaryObserved
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
