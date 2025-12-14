package emitter

import (
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/concurrent"
	"github.com/0xsoniclabs/daphne/daphne/consensus"
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
	Leader consensus.ValidatorId
}

type PartialSynchronyEmitter struct {
	dag       model.Dag
	committee *consensus.Committee
	creator   consensus.ValidatorId
	leader    consensus.ValidatorId

	lastEmittedSeq uint32

	channel Channel

	timeout         time.Duration
	timeoutJob      *concurrent.Job
	timeoutOccurred atomic.Bool

	stateMutex sync.Mutex
}

func (f *PartialSynchronyEmitterFactory) NewEmitter(channel Channel, dag model.Dag, creator consensus.ValidatorId) Emitter {
	return newPartialSynchronyEmitter(dag, f.Committee, creator, f.Leader, f.Timeout, channel)
}

func newPartialSynchronyEmitter(
	dag model.Dag,
	committee *consensus.Committee,
	creator consensus.ValidatorId,
	leader consensus.ValidatorId,
	timeout time.Duration,
	channel Channel,
) *PartialSynchronyEmitter {
	e := &PartialSynchronyEmitter{
		dag:       dag,
		committee: committee,
		creator:   creator,
		leader:    leader,
		timeout:   timeout,
		channel:   channel,
	}
	e.startTimeoutTimer()
	return e
}

func (f *PartialSynchronyEmitterFactory) String() string {
	return "psynchrony_" + f.Timeout.String()
}

func (e *PartialSynchronyEmitter) OnChange() {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	if e.channel == nil {
		return
	}

	dagHeads := e.dag.GetHeads()
	if e.shouldEmit(dagHeads) {
		e.channel.Emit(e.dag.GetHeads())

		e.lastEmittedSeq++
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
		case <-time.After(e.timeout):
			e.timeoutOccurred.Store(true)
			go e.OnChange()
			return
		case <-stop:
			return
		}
	})
}

func (e *PartialSynchronyEmitter) shouldEmit(dagHeads map[consensus.ValidatorId]*model.Event) bool {
	if e.lastEmittedSeq == 0 {
		return true
	}

	if !e.observesLatestEmisstion(dagHeads) || !e.observesQuorumOfPrevRound(dagHeads) {
		return false
	}

	return e.timeoutOccurred.Load() || e.supportsTheLeader(dagHeads)
}

func (e *PartialSynchronyEmitter) observesLatestEmisstion(dagHeads map[consensus.ValidatorId]*model.Event) bool {
	lastObservedEvent, observedGenesisEmission := dagHeads[e.creator]

	lastObservedEventSeq := uint32(0)
	if observedGenesisEmission {
		lastObservedEventSeq = lastObservedEvent.Seq()
	}

	return lastObservedEventSeq >= e.lastEmittedSeq
}

func (e *PartialSynchronyEmitter) supportsTheLeader(dagHeads map[consensus.ValidatorId]*model.Event) bool {
	switch e.lastEmittedSeq % 3 {
	case 1:
		// Emit if you have received the primary leader
		_, primaryLeaderExists := dagHeads[e.leader]
		return primaryLeaderExists

	case 2:
		// Emit if a quorum of events reaches the primary leader
		voteCounter := consensus.NewVoteCounter(e.committee)
		primaryLeader := dagHeads[e.leader].SelfParent()
		for _, head := range dagHeads {
			if e.dag.Reaches(head, primaryLeader) {
				voteCounter.Vote(head.Creator())
			}
		}
		return voteCounter.IsQuorumReached()

	default:
		return true
	}
}

func (e *PartialSynchronyEmitter) observesQuorumOfPrevRound(dagHeads map[consensus.ValidatorId]*model.Event) bool {
	eligibleParents := sets.Filter(sets.New(slices.Collect(maps.Values(dagHeads))...), func(event *model.Event) bool {
		return event.Seq() == e.lastEmittedSeq
	})

	voteCounter := consensus.NewVoteCounter(e.committee)
	for parent := range eligibleParents.All() {
		voteCounter.Vote(parent.Creator())
	}

	return voteCounter.IsQuorumReached()
}
