package dag

import (
	"fmt"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type ObservesQuorumOfLatestEmission[P payload.Payload] struct {
	emitter   *Emitter[P]
	committee *consensus.Committee
}

func (oqlc *ObservesQuorumOfLatestEmission[P]) Init() {
}

func (olec *ObservesQuorumOfLatestEmission[P]) Evaluate() bool {
	if olec.emitter.lastEmittedSeq == 0 {
		return true
	}
	headSet := sets.New(slices.Collect(maps.Values(olec.emitter.dag.GetHeads()))...)
	eligibleParents := sets.Filter(headSet, func(e *model.Event) bool {
		return e.Seq() == olec.emitter.lastEmittedSeq
	})
	voteCounter := consensus.NewVoteCounter(olec.committee)
	for parent := range eligibleParents.All() {
		voteCounter.Vote(parent.Creator())
	}
	return voteCounter.IsQuorumReached()
}

type TimedOutCondition[P payload.Payload] struct {
	cond     Cond
	duration time.Duration
	emitter  *Emitter[P]
	done     atomic.Bool

	mu sync.Mutex
}

func (toc *TimedOutCondition[P]) Init() {
	toc.cond.Init()
	toc.done = atomic.Bool{}
	seq := toc.emitter.lastEmittedSeq
	fmt.Println("Scheduling timeout for seq ", seq, " time ", time.Now())
	go func() {
		time.Sleep(toc.duration)

		fmt.Print("Finished sleeping for seq ", seq, " time ", time.Now())

		if seq != toc.emitter.lastEmittedSeq {
			fmt.Print(" - rejected due to seq change ", toc.emitter.lastEmittedSeq, " instead of ", seq, "\n")
			return
		}

		toc.mu.Lock()
		defer toc.mu.Unlock()

		toc.done.Store(true)
		toc.emitter.AttemptEmission()
	}()
}

func (toc *TimedOutCondition[P]) Evaluate() bool {
	return toc.done.Load() || toc.cond.Evaluate()
}

type SeesLeaderEnoughCondition[P payload.Payload] struct {
	emitter   *Emitter[P]
	leader    consensus.ValidatorId
	committee *consensus.Committee
}

func (slc *SeesLeaderEnoughCondition[P]) Init() {
}

func (slc *SeesLeaderEnoughCondition[P]) Evaluate() bool {
	switch slc.emitter.lastEmittedSeq % 3 {

	case 1:
		// have you received the primary leader ?
		heads := slc.emitter.dag.GetHeads()
		_, primaryLeaderExists := heads[slc.leader]
		return primaryLeaderExists

	case 2:
		// do you strongly see the primary leader ?
		heads := slc.emitter.dag.GetHeads()
		voteCounter := consensus.NewVoteCounter(slc.committee)
		primaryLeader := heads[slc.leader].SelfParent()
		for _, head := range heads {
			if slc.emitter.dag.Reaches(head, primaryLeader) {
				voteCounter.Vote(head.Creator())
			}
		}
		return voteCounter.IsQuorumReached()

	default:
		return true
	}
}

func TestEmitter(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)

	channel := broadcast.NewMockChannel[EventMessage[payload.Transactions]](ctrl)

	transactionSource := consensus.NewMockTransactionProvider(ctrl)

	lineup := txpool.NewMockLineup(ctrl)
	lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()
	transactionSource.EXPECT().GetCandidateLineup().Return(lineup).AnyTimes()

	committee := consensus.NewUniformCommittee(4)
	dag := model.NewDag(committee)

	const primaryLeader = consensus.ValidatorId(1)

	emitter := NewEmitter(0, dag, payload.RawProtocol{}, transactionSource, channel)

	mysticetiCondition := &TimedOutCondition[payload.Transactions]{
		cond: &SeesLeaderEnoughCondition[payload.Transactions]{
			emitter:   emitter,
			leader:    primaryLeader,
			committee: committee,
		},
		duration: 500 * time.Millisecond,
		emitter:  emitter,
	}

	emitter.AddConditions(
		&ObservesLatestEmissionCondition[payload.Transactions]{
			emitter: emitter,
		},
		&ObservesQuorumOfLatestEmission[payload.Transactions]{
			emitter:   emitter,
			committee: committee,
		},
		mysticetiCondition,
	)

	channel.EXPECT().Broadcast(gomock.Any()).Do(func(msg EventMessage[payload.Transactions]) {
		dag.AddEvent(msg.nested)
	}).Times(3)

	synctest.Test(t, func(t *testing.T) {
		emitter.Ignite()
		emitter.AttemptEmission() // genesis (1st emission)
		require.Len(dag.GetHeads(), 1)
		emitter.AttemptEmission()

		e1_0 := dag.GetHeads()[0]
		e1_1 := dag.AddEvent(model.EventMessage{Creator: 1, Parents: []model.EventId{}})[0]
		emitter.AttemptEmission()
		e1_2 := dag.AddEvent(model.EventMessage{Creator: 2, Parents: []model.EventId{}})[0]
		emitter.AttemptEmission() // quorum of round 1 parents (2nd emission)
		e1_3 := dag.AddEvent(model.EventMessage{Creator: 3, Parents: []model.EventId{}})[0]
		emitter.AttemptEmission()

		_ = dag.GetHeads()[0]
		_ = dag.AddEvent(model.EventMessage{Creator: 1, Parents: []model.EventId{e1_1.EventId(), e1_0.EventId(), e1_2.EventId()}})[0]
		emitter.AttemptEmission()

		_ = dag.AddEvent(model.EventMessage{Creator: 2, Parents: []model.EventId{e1_2.EventId(), e1_3.EventId(), e1_0.EventId()}})[0]
		time.Sleep(600 * time.Millisecond) // wait for timeout

		emitter.Stop()

		// fmt.Print("Throwaway", e2_0, e2_1, e2_2, e1_3)

		time.Sleep(1 * time.Hour)
	})

}

func TestEra(t *testing.T) {
	generator := func() condition {
		timeout := atomic.Bool{}
		go func() {
			time.Sleep(500 * time.Millisecond)
			timeout.Store(true)
			fmt.Printf("TRIGGERING TIMEOUT %p\n", &timeout)
		}()
		fmt.Printf("SPAWNED A TIMEOUT %p\n", &timeout)
		return func() bool {
			fmt.Printf("CHECKING TIMEOUT %p, %v\n", &timeout, timeout.Load())
			return timeout.Load()
		}
	}

	synctest.Test(t, func(t *testing.T) {
		require := require.New(t)

		cond1 := generator()
		cond2 := generator()
		time.Sleep(100 * time.Millisecond)
		require.False(cond1())
		require.False(cond2())

		time.Sleep(450 * time.Millisecond)
		require.True(cond1())
		require.True(cond2())

		time.Sleep(1 * time.Hour)
	})

}
