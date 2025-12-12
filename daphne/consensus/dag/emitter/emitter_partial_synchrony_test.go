package emitter

import (
	"maps"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestEmitter_PartialSynchronyLikeConditions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		require := require.New(t)
		ctrl := gomock.NewController(t)

		emitChannel := NewMockChannel(ctrl)

		const primaryLeader, creator = consensus.ValidatorId(1), consensus.ValidatorId(0)
		const timeoutDuration = 500 * time.Millisecond

		committee := consensus.NewUniformCommittee(4)
		dag := model.NewDag(committee)

		observesLatestEmissionCondition := &observesLatestEmissionCondition{}
		observesQuorumOfPrevRound := &observesQuorumOfLastRound{committee: committee}
		partialSynchronyCondition := NewOrCondition( // Mysticeti-like partial synchrony condition
			&believesInLeader{leader: primaryLeader, committee: committee},
			&timeoutCondition{duration: timeoutDuration},
		)

		// Expect total of 3 emissions, update the DAG accordingly
		emitChannel.EXPECT().Emit(gomock.Any()).Do(func(parents []model.EventId) {
			msg := model.EventMessage{Creator: creator, Parents: parents}
			dag.AddEvent(msg)
		}).Times(3)

		emitter := StartNewEmitter(0, dag, emitChannel,
			NewAndCondition(
				observesLatestEmissionCondition,
				observesQuorumOfPrevRound,
				partialSynchronyCondition,
			))

		emitter.AttemptEmission() // genesis (1st emission)
		require.Equal(uint32(1), emitter.getLastEmittedSeq())

		// Attempt another emission without updating the DAG, should not emit
		emitter.AttemptEmission()
		require.Equal(uint32(1), emitter.getLastEmittedSeq())

		// e_#seq_#validator

		e1_0 := dag.GetHeads()[0]

		// Add another genesis event (primary leader), but should not emit due to lack of quorum
		e1_1 := dag.AddEvent(model.EventMessage{Creator: 1, Parents: []model.EventId{}})[0]
		emitter.AttemptEmission()
		require.Equal(uint32(1), emitter.getLastEmittedSeq())

		time.Sleep(timeoutDuration / 2)
		// Quorum of round 1 parents observed, primary leader observed + timer NOT triggered (2nd emission)
		e1_2 := dag.AddEvent(model.EventMessage{Creator: 2, Parents: []model.EventId{}})[0]
		emitter.AttemptEmission()
		require.Equal(uint32(2), emitter.getLastEmittedSeq())
		require.Equal(uint32(2), dag.GetHeads()[0].Seq())

		// Last genesis event, does not trigger any conditions.
		e1_3 := dag.AddEvent(model.EventMessage{Creator: 3, Parents: []model.EventId{}})[0]
		emitter.AttemptEmission()
		require.Equal(uint32(2), emitter.getLastEmittedSeq())

		// Primary creator event with Seq 2 is added, no conditions met.
		dag.AddEvent(model.EventMessage{Creator: 1, Parents: []model.EventId{e1_1.EventId(), e1_0.EventId(), e1_2.EventId()}})
		emitter.AttemptEmission()
		require.Equal(uint32(2), emitter.getLastEmittedSeq())

		// Quorum of round 2 parents reached, but no primary leader obsesrved by quorum of them yet.
		dag.AddEvent(model.EventMessage{Creator: 2, Parents: []model.EventId{e1_2.EventId(), e1_3.EventId(), e1_0.EventId()}})

		time.Sleep(timeoutDuration + 100*time.Millisecond)
		// Quorum of round 2 parents reached + timeout triggered and (leader observed by quorum) condition NOT met (3rd emission)

		emitter.Stop()
		time.Sleep(1 * time.Hour)
	})

}

type observesQuorumOfLastRound struct {
	committee *consensus.Committee
}

func (*observesQuorumOfLastRound) Reset(*Emitter, map[consensus.ValidatorId]*model.Event) {}

func (o *observesQuorumOfLastRound) Evaluate(emitter *Emitter) bool {
	// Always true for the genesis emission
	if emitter.getLastEmittedSeq() == 0 {
		return true
	}
	headSet := sets.New(slices.Collect(maps.Values(emitter.getDag().GetHeads()))...)
	eligibleParents := sets.Filter(headSet, func(e *model.Event) bool {
		return e.Seq() == emitter.getLastEmittedSeq()
	})
	voteCounter := consensus.NewVoteCounter(o.committee)
	for parent := range eligibleParents.All() {
		voteCounter.Vote(parent.Creator())
	}
	return voteCounter.IsQuorumReached()
}

func (*observesQuorumOfLastRound) Stop() {}

type believesInLeader struct {
	leader    consensus.ValidatorId
	committee *consensus.Committee
}

func (*believesInLeader) Reset(*Emitter, map[consensus.ValidatorId]*model.Event) {}

func (b *believesInLeader) Evaluate(emitter *Emitter) bool {
	switch emitter.getLastEmittedSeq() % 3 {

	case 1:
		// have you received the primary leader ?
		heads := emitter.getDag().GetHeads()
		_, primaryLeaderExists := heads[b.leader]
		return primaryLeaderExists

	case 2:
		// do you strongly see the primary leader ?
		heads := emitter.getDag().GetHeads()
		voteCounter := consensus.NewVoteCounter(b.committee)
		primaryLeader := heads[b.leader].SelfParent()
		for _, head := range heads {
			if emitter.dag.Reaches(head, primaryLeader) {
				voteCounter.Vote(head.Creator())
			}
		}
		return voteCounter.IsQuorumReached()

	default:
		return true
	}
}

func (*believesInLeader) Stop() {}
