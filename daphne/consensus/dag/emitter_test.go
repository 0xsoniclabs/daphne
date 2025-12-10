package dag

import (
	"fmt"
	"maps"
	"slices"
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
	const waveLength = 3
	generators := []conditionGenerator[payload.Transactions]{
		func(e *Emitter[payload.Transactions]) condition {
			return e.ObservesLatestEmission()
		},
		func(emitter *Emitter[payload.Transactions]) condition {
			return func() bool {
				if emitter.lastEmittedSeq == 0 {
					return true
				}
				headSet := sets.New(slices.Collect(maps.Values(emitter.dag.GetHeads()))...)
				eligibleParents := sets.Filter(headSet, func(e *model.Event) bool {
					return e.Seq() == emitter.lastEmittedSeq
				})
				voteCounter := consensus.NewVoteCounter(committee)
				for parent := range eligibleParents.All() {
					voteCounter.Vote(parent.Creator())
				}
				return voteCounter.IsQuorumReached()
			}
		},
		func(emitter *Emitter[payload.Transactions]) condition {
			timeout := atomic.Bool{}
			seq := emitter.lastEmittedSeq
			go func() {
				time.Sleep(500 * time.Millisecond)
				timeout.Store(true)

				// fmt.Printf("TRIGGERING TIMEOUT FOR SEQ %d timeout %p\n", seq, &timeout)
				if seq != emitter.lastEmittedSeq {
					return
				}
				emitter.AttemptEmission()
			}()
			// fmt.Printf("SPAWNED A TIMEOUT %d %p\n", seq, &timeout)
			return func() bool {
				switch emitter.lastEmittedSeq % waveLength {
				default:
					return true
				case 1:
					// have you received the primary leader ?
					// Let's say that the primary leader is the validator with ID 1.
					heads := emitter.dag.GetHeads()
					_, primaryLeaderExists := heads[primaryLeader]
					return primaryLeaderExists || timeout.Load()

					// do you strongly see the primary leader ?
				case 2:
					heads := emitter.dag.GetHeads()
					voteCounter := consensus.NewVoteCounter(committee)
					primaryLeader := heads[primaryLeader].SelfParent()
					for _, head := range heads {
						if emitter.dag.Reaches(head, primaryLeader) {
							voteCounter.Vote(head.Creator())
						}
					}
					// fmt.Print(" - vote sum -  ", voteCounter.GetVoteSum(), " - ")
					// fmt.Printf(" timeout - %p - ", &timeout)
					return voteCounter.IsQuorumReached() || timeout.Load()
				}
			}
		},
	}

	emitter := NewEmitter(0, dag, payload.RawProtocol{}, transactionSource, channel, generators)

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
		time.Sleep(1000 * time.Millisecond) // wait for timeout

		emitter.AttemptEmission() // quorum of round 2 parents that see the  (3rd emission)

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
