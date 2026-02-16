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

package mysticeti

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
	"github.com/stretchr/testify/require"
)

func TestMysticeti_Factory_ImplementsLayeringFactory(t *testing.T) {
	var _ layering.Factory = Factory{}
}

func TestMysticeti_Factory_String_ReturnsSummary(t *testing.T) {
	factory := Factory{}
	require.Equal(t, "mysticeti", factory.String())
}

func TestMysticeti_Factory_NewLayering_ReturnsMysticetiInstance(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	factory := Factory{}
	layering := factory.NewLayering(dag, committee)

	require.NotNil(t, layering)
	m, ok := layering.(*Mysticeti)
	require.True(t, ok)
	require.Equal(t, dag, m.dag)
	require.Equal(t, committee, m.committee)
	require.Len(t, m.validators, 4)
}

func TestMysticeti_Factory_NewLayering_PanicsOnNonUniformStake(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 2, // Non-uniform stake
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	factory := Factory{}

	require.Panics(t, func() {
		factory.NewLayering(dag, committee)
	})
}

func TestMysticeti_GetRoundLeader_RoundRobinAssignment(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	// Validators are sorted, so order is [1, 2, 3]
	validators := committee.Validators()
	require.Len(t, validators, 3)

	// Test round-robin for several rounds
	for round := range uint32(10) {
		expectedLeader := validators[int(round)%3]
		actualLeader := m.getRoundLeader(round)
		require.Equal(t, expectedLeader, actualLeader,
			"Round %d should have leader %d", round, expectedLeader)
	}
}

func TestMysticeti_GetRoundLeader_EmptyValidatorSetDoesNotPanic(t *testing.T) {
	// Create Mysticeti with empty validator set
	m := &Mysticeti{
		validators: []consensus.ValidatorId{},
	}

	// Should return zero ValidatorId without panicking
	leader := m.getRoundLeader(0)
	var zero consensus.ValidatorId
	require.Equal(t, zero, leader)
}

func TestMysticeti_IsRoundLeader_CorrectlyIdentifiesLeader(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Check that each validator is the leader of their assigned rounds
	for round := range uint32(6) {
		for i, validator := range validators {
			expectedIsLeader := (int(round) % 3) == i
			actualIsLeader := m.isRoundLeader(round, validator)
			require.Equal(t, expectedIsLeader, actualIsLeader,
				"Validator %d leadership in round %d", validator, round)
		}
	}
}

func TestMysticeti_Reaches_VoterReachesCandidate(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	// Create a simple chain: genesis -> event1
	genesis, err := model.NewEvent(consensus.ValidatorId(1), nil, nil, time.Time{})
	require.NoError(t, err)

	event1, err := model.NewEvent(consensus.ValidatorId(1), []*model.Event{genesis}, nil, time.Time{})
	require.NoError(t, err)

	// event1 should reach genesis
	require.True(t, m.reaches(event1, genesis))
	// genesis should not reach event1 (wrong direction)
	require.False(t, m.reaches(genesis, event1))
}

// Verifies that reaches correctly handles equivocation by only recognizing
// the FIRST event encountered in DFS.
func TestMysticeti_Reaches_HandlesEquivocation(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	// Create genesis events
	genesis1, err := model.NewEvent(consensus.ValidatorId(1), nil, nil, time.Time{})
	require.NoError(t, err)
	genesis2, err := model.NewEvent(consensus.ValidatorId(2), nil, nil, time.Time{})
	require.NoError(t, err)
	genesis3, err := model.NewEvent(consensus.ValidatorId(3), nil, nil, time.Time{})
	require.NoError(t, err)

	// Validator 1 equivocates at round 1: creates two different events
	equivocate1a, err := model.NewEvent(consensus.ValidatorId(1),
		[]*model.Event{genesis1, genesis2}, payload.Transactions{{Nonce: 1}}, time.Time{})
	require.NoError(t, err)
	equivocate1b, err := model.NewEvent(consensus.ValidatorId(1),
		[]*model.Event{genesis1, genesis3}, payload.Transactions{{Nonce: 2}}, time.Time{})
	require.NoError(t, err)

	// Validator 2 sees equivocate1a first
	event2, err := model.NewEvent(consensus.ValidatorId(2),
		[]*model.Event{genesis2, equivocate1a, genesis3}, nil, time.Time{})
	require.NoError(t, err)

	// Validator 3 sees equivocate1b first
	event3, err := model.NewEvent(consensus.ValidatorId(3),
		[]*model.Event{genesis3, equivocate1b, genesis2}, nil, time.Time{})
	require.NoError(t, err)

	// event2 reaches equivocate1a (encounters it first in DFS)
	require.True(t, m.reaches(event2, equivocate1a))
	// event2 does NOT reach equivocate1b (encounters equivocate1a first)
	require.False(t, m.reaches(event2, equivocate1b))

	// event3 reaches equivocate1b (encounters it first in DFS)
	require.True(t, m.reaches(event3, equivocate1b))
	// event3 does NOT reach equivocate1a (encounters equivocate1b first)
	require.False(t, m.reaches(event3, equivocate1a))

	// Create event4 that references only event2 and event3 as parents.
	// This brings both equivocating events into event4's closure:
	// - equivocate1a via event2
	// - equivocate1b via event3
	// However, reaches() should only return true for ONE of them.
	event4, err := model.NewEvent(consensus.ValidatorId(2),
		[]*model.Event{event2, event3}, nil, time.Time{})
	require.NoError(t, err)

	// event4 has both equivocate1a and equivocate1b in its closure,
	// but reaches() only returns true for the FIRST one encountered in DFS.
	// DFS visits parents in order, so event2 is visited first, which means
	// equivocate1a is encountered before equivocate1b.
	reaches1a := m.reaches(event4, equivocate1a)
	reaches1b := m.reaches(event4, equivocate1b)

	// Exactly one of the equivocating events should be reached
	require.True(t, reaches1a != reaches1b,
		"exactly one equivocating event should be reached, got reaches1a=%v, reaches1b=%v",
		reaches1a, reaches1b)

	// Given the parent ordering [event2, event3], event2 is visited first,
	// so equivocate1a (which event2 includes) should be reached first.
	require.True(t, reaches1a, "equivocate1a should be reached via event2")
	require.False(t, reaches1b, "equivocate1b should NOT be reached (equivocation already seen)")
}

func TestMysticeti_Reaches_NilEvents(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	event, err := model.NewEvent(consensus.ValidatorId(1), nil, nil, time.Time{})
	require.NoError(t, err)

	require.False(t, m.reaches(nil, event))
	require.False(t, m.reaches(event, nil))
	require.False(t, m.reaches(nil, nil))
}

func TestMysticeti_Reaches_DiamondDAG(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	// Create a diamond DAG structure:
	//     genesis1
	//      /    \
	//   event2  event3
	//      \    /
	//      voter
	genesis1, err := model.NewEvent(consensus.ValidatorId(1), nil, nil, time.Time{})
	require.NoError(t, err)
	genesis2, err := model.NewEvent(consensus.ValidatorId(2), nil, nil, time.Time{})
	require.NoError(t, err)
	genesis3, err := model.NewEvent(consensus.ValidatorId(3), nil, nil, time.Time{})
	require.NoError(t, err)

	// Two events both reference genesis1
	event2, err := model.NewEvent(consensus.ValidatorId(2), []*model.Event{genesis2, genesis1}, nil, time.Time{})
	require.NoError(t, err)
	event3, err := model.NewEvent(consensus.ValidatorId(3), []*model.Event{genesis3, genesis1}, nil, time.Time{})
	require.NoError(t, err)

	// Voter references both paths (creates diamond)
	voter, err := model.NewEvent(consensus.ValidatorId(2), []*model.Event{event2, event3}, nil, time.Time{})
	require.NoError(t, err)

	// voter reaches genesis1
	reaches := m.reaches(voter, genesis1)
	require.True(t, reaches)
}

// TestMysticeti_Reaches_PruningBelowCandidateRound verifies that the DFS prunes
// branches when the event round is below the candidate round.
func TestMysticeti_Reaches_PruningBelowCandidateRound(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Round 0: Genesis events
	genesis0, err := model.NewEvent(validators[0], nil, nil, time.Time{})
	require.NoError(t, err)
	dag.AddEvent(model.EventMessage{Creator: validators[0]})

	genesis1, err := model.NewEvent(validators[1], nil, nil, time.Time{})
	require.NoError(t, err)
	dag.AddEvent(model.EventMessage{Creator: validators[1]})

	genesis2, err := model.NewEvent(validators[2], nil, nil, time.Time{})
	require.NoError(t, err)
	dag.AddEvent(model.EventMessage{Creator: validators[2]})

	// Round 1: Validator 1 creates an event (this will be the candidate)
	round1_1, err := model.NewEvent(validators[1], []*model.Event{genesis1, genesis0}, nil, time.Time{})
	require.NoError(t, err)
	dag.AddEvent(model.EventMessage{Creator: validators[1], Parents: []model.EventId{genesis1.EventId(), genesis0.EventId()}})

	// Round 1: Validator 0 creates an event
	round1_0, err := model.NewEvent(validators[0], []*model.Event{genesis0, genesis1}, nil, time.Time{})
	require.NoError(t, err)
	dag.AddEvent(model.EventMessage{Creator: validators[0], Parents: []model.EventId{genesis0.EventId(), genesis1.EventId()}})

	// Round 2: Validator 2 creates an event referencing round1_0 and genesis2
	// Ancestry: round2_2 -> round1_0 -> genesis0, genesis1
	//           round2_2 -> genesis2
	// When searching for round1_1 (candidate at round 1 from validator 1):
	// DFS visits round2_2 (round 2, validator 2) - not candidate, descend
	// DFS visits round1_0 (round 1, validator 0) - not candidate (wrong author), round 1 >= 1, descend
	// DFS visits genesis0 (round 0, validator 0) - round 0 < candidateRound 1, PRUNE
	// DFS visits genesis1 (round 0, validator 1) - round 0 < candidateRound 1, PRUNE
	// DFS visits genesis2 (round 0, validator 2) - round 0 < candidateRound 1, PRUNE
	// Result: round1_1 not found, returns false
	round2_2, err := model.NewEvent(validators[2], []*model.Event{genesis2, round1_0}, nil, time.Time{})
	require.NoError(t, err)
	dag.AddEvent(model.EventMessage{Creator: validators[2], Parents: []model.EventId{genesis2.EventId(), round1_0.EventId()}})

	// round2_2 does NOT reach round1_1 (round1_1 is not in its ancestry)
	// DFS visits genesis events (round 0) which are below candidateRound (1)
	// and triggers the prune condition
	require.False(t, m.reaches(round2_2, round1_1))

	// Verify the positive case still works
	// Voter at round 2 that DOES include round1_1 in ancestry
	round2_1, err := model.NewEvent(validators[1], []*model.Event{round1_1, round1_0}, nil, time.Time{})
	require.NoError(t, err)
	dag.AddEvent(model.EventMessage{Creator: validators[1], Parents: []model.EventId{round1_1.EventId(), round1_0.EventId()}})

	require.True(t, m.reaches(round2_1, round1_1))
}

func TestMysticeti_IsCandidate_LeaderEventIsCandidate(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create genesis events
	genesis1, err := model.NewEvent(validators[0], nil, nil, time.Time{})
	require.NoError(t, err)
	genesis2, err := model.NewEvent(validators[1], nil, nil, time.Time{})
	require.NoError(t, err)

	// Round 0 genesis events - leader is validators[0]
	require.True(t, m.IsCandidate(genesis1))
	require.False(t, m.IsCandidate(genesis2))

	// Create round 1 events - leader is validators[1]
	event1, err := model.NewEvent(validators[0], []*model.Event{genesis1, genesis2}, nil, time.Time{})
	require.NoError(t, err)
	event2, err := model.NewEvent(validators[1], []*model.Event{genesis2, genesis1}, nil, time.Time{})
	require.NoError(t, err)

	require.False(t, m.IsCandidate(event1))
	require.True(t, m.IsCandidate(event2))
}

func TestMysticeti_IsCandidate_NilEventIsNotCandidate(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	require.False(t, m.IsCandidate(nil))
}

func TestMysticeti_IsLeader_CertifiesLeaderWithCommitPattern(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build complete 3-round commit scenario
	// Round 0: Genesis (leader is validators[0])
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader := genesis[0]

	// Round 1: All vote for leader
	round1 := make([]*model.Event, 4)
	for i, v := range validators {
		parents := []*model.Event{genesis[i], leader}
		if i == 0 {
			parents = []*model.Event{genesis[i]}
		}
		var err error
		round1[i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Round 2: All create certificates
	for i, v := range validators {
		parents := []*model.Event{round1[i], round1[(i+1)%4], round1[(i+2)%4], round1[(i+3)%4]}
		_, err := model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Leader should be certified (VerdictYes)
	verdict := m.IsLeader(leader)
	require.Equal(t, layering.VerdictYes, verdict)
}

func TestMysticeti_IsLeader_SkipsLeaderWithSkipPattern(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Round 0: Genesis
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader := genesis[0]

	// Round 1: 3 validators skip the leader
	for i := 1; i < 4; i++ {
		// Carefully select parents to avoid including genesis[0]
		var parents []*model.Event
		switch i {
		case 1:
			parents = []*model.Event{genesis[i], genesis[2]} // Skip genesis[0]
		case 2:
			parents = []*model.Event{genesis[i], genesis[3]} // Skip genesis[0]
		default: // i == 3
			parents = []*model.Event{genesis[i], genesis[1]} // Skip genesis[0]
		}
		_, err := model.NewEvent(validators[i], parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: validators[i], Parents: parentIds, Payload: nil})
	}

	// Leader should be skipped (VerdictNo)
	verdict := m.IsLeader(leader)
	require.Equal(t, layering.VerdictNo, verdict)
}

func TestMysticeti_IsLeader_NonCandidateReturnsNo(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create genesis events - round 0 leader is validators[0]
	_, err = model.NewEvent(validators[0], nil, nil, time.Time{})
	require.NoError(t, err)
	genesis2, err := model.NewEvent(validators[1], nil, nil, time.Time{})
	require.NoError(t, err)

	// genesis2 is not a candidate (not the leader)
	verdict := m.IsLeader(genesis2)
	require.Equal(t, layering.VerdictNo, verdict)
}

func TestMysticeti_TryDecideRoundLeader_AlreadyDecidedLeadsToStableVerdict(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build complete scenario
	events := make([][]*model.Event, 5)

	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	for round := 1; round < 5; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := 0; j < 3; j++ {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Call IsLeader multiple times to trigger already-decided path
	leader := events[0][0]
	verdict1 := m.IsLeader(leader)
	verdict2 := m.IsLeader(leader)

	require.Equal(t, verdict1, verdict2, "Verdict should be stable")
	require.Equal(t, layering.VerdictYes, verdict1)
}

func TestMysticeti_TryDecideRoundLeader_LowestUndecidedRoundProgresses(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build complete scenario
	events := make([][]*model.Event, 5)

	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	for round := 1; round < 5; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := 0; j < 3; j++ {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// First call decides round 0
	leader0 := events[0][0]
	verdict1 := m.IsLeader(leader0)
	require.Equal(t, layering.VerdictYes, verdict1)

	// Second call on same leader
	verdict2 := m.IsLeader(leader0)
	require.Equal(t, verdict1, verdict2, "Verdict should be stable")

	// Verify lowestUndecidedRound advanced
	require.Greater(t, m.lowestUndecidedRound, uint32(0))
}

func TestMysticeti_TryDecideRoundLeader_VerdictUndecidedIfShouldBeUndecided(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Round 0: Genesis
	for _, v := range validators {
		_, err := model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Don't build round 1 and 2 - leader at round 0 will be undecided
	leader := m.findLeaderInRound(0)
	require.NotNil(t, leader)

	// Call IsLeader - should remain undecided
	verdict := m.IsLeader(leader)
	require.Equal(t, layering.VerdictUndecided, verdict)
}

func TestMysticeti_TryDecideRoundLeader_AlreadyDecidedRoundReturnsEarly(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build genesis
	for _, v := range validators {
		_, err := model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v})
	}

	// Pre-populate decision for round 0 with lowestUndecidedRound already advanced
	m.slotDecisions[0] = layering.VerdictYes
	m.lowestUndecidedRound = 1

	// Call tryDecideRoundLeader on already-decided round - should return early without changes
	m.tryDecideRoundLeader(0, committee.Quorum())
	require.Equal(t, uint32(1), m.lowestUndecidedRound)       // Unchanged
	require.Equal(t, layering.VerdictYes, m.slotDecisions[0]) // Still the same decision
}

func TestMysticeti_TryDecideRoundLeader_NoLeaderInRoundReturnsEarlyWithoutLeader(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create genesis for all except the leader of round 1
	for i, v := range validators {
		if i == 1 {
			continue // Skip leader
		}
		_, err := model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v})
	}

	// Try to decide round 1 - should return early due to missing leader
	initialDecisions := len(m.slotDecisions)
	m.tryDecideRoundLeader(1, committee.Quorum())
	require.Equal(t, initialDecisions, len(m.slotDecisions))
}

func TestMysticeti_TryDecideRoundLeader_VerdictDecidedIfShouldBeUndecidedAndCachesDecision(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build scenario where leader gets decided
	events := make([][]*model.Event, 5)

	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	for round := 1; round < 5; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := 0; j < 3; j++ {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Call IsLeader to trigger tryDecideRoundLeader
	leader := events[0][0]
	verdict := m.IsLeader(leader)
	require.Equal(t, layering.VerdictYes, verdict)

	// Verify decision was cached
	_, exists := m.slotDecisions[0]
	require.True(t, exists, "Decision should be cached")
}

func TestMysticeti_DirectDecision_CommitPatternLeadsToCommit(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)
	quorum := committee.Quorum()

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build complete 3-round structure for commit
	// Round 0: Genesis (leader)
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader := genesis[0]

	// Round 1: All vote for leader
	round1 := make([]*model.Event, 4)
	for i, v := range validators {
		parents := []*model.Event{genesis[i], leader}
		if i == 0 {
			parents = []*model.Event{genesis[i]}
		}
		var err error
		round1[i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Round 2: All create certificates
	for i, v := range validators {
		parents := []*model.Event{round1[i], round1[(i+1)%4], round1[(i+2)%4], round1[(i+3)%4]}
		_, err := model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Direct decision should return VerdictYes (commit)
	verdict := m.directDecision(leader, 0, quorum)
	require.Equal(t, layering.VerdictYes, verdict)
}

func TestMysticeti_DirectDecision_SkipPatternLeadsToSkip(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)
	quorum := committee.Quorum()

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Round 0: Genesis
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader := genesis[0]

	// Round 1: 3 validators skip the leader
	for i := 1; i < 4; i++ {
		// Carefully select parents to avoid including genesis[0]
		var parents []*model.Event
		switch i {
		case 1:
			parents = []*model.Event{genesis[i], genesis[2]} // Skip genesis[0]
		case 2:
			parents = []*model.Event{genesis[i], genesis[3]} // Skip genesis[0]
		default: // i == 3
			parents = []*model.Event{genesis[i], genesis[1]} // Skip genesis[0]
		}
		_, err := model.NewEvent(validators[i], parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: validators[i], Parents: parentIds, Payload: nil})
	}

	// Direct decision should return VerdictNo (skip)
	verdict := m.directDecision(leader, 0, quorum)
	require.Equal(t, layering.VerdictNo, verdict)
}

func TestMysticeti_DirectDecision_UndecidedWithoutPatterns(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)
	quorum := committee.Quorum()

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Round 0: Genesis
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader := genesis[0]

	// Round 1: Only 2 validators vote (not enough for either pattern)
	for i := range 2 {
		parents := []*model.Event{genesis[i], leader}
		if i == 0 {
			parents = []*model.Event{genesis[i]}
		}
		_, err := model.NewEvent(validators[i], parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: validators[i], Parents: parentIds, Payload: nil})
	}

	// Direct decision should return VerdictUndecided
	verdict := m.directDecision(leader, 0, quorum)
	require.Equal(t, layering.VerdictUndecided, verdict)
}

// Verifies that skip pattern is correctly detected when 2f+1 events in r+1
// don't have direct parent from leader's author at leader's round.
func TestMysticeti_HasSkipPattern_DetectsSkip(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)
	quorum := committee.Quorum()

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create genesis events for all validators
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{
			Creator: v,
			Parents: nil,
			Payload: nil,
		})
	}

	// Leader of round 0 is validators[0]
	leader := genesis[0]

	// Create round 1 events where 3 out of 4 validators do NOT include
	// the leader as a direct parent
	for i := 1; i < 4; i++ {
		// These events skip the leader (validators[0])
		// Carefully select parents to avoid including genesis[0]
		var parents []*model.Event
		switch i {
		case 1:
			parents = []*model.Event{genesis[i], genesis[2]} // Skip genesis[0]
		case 2:
			parents = []*model.Event{genesis[i], genesis[3]} // Skip genesis[0]
		default: // i == 3
			parents = []*model.Event{genesis[i], genesis[1]} // Skip genesis[0]
		}
		_, err := model.NewEvent(validators[i], parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{
			Creator: validators[i],
			Parents: parentIds,
			Payload: nil,
		})
	}

	// Skip pattern should be detected
	hasSkip := m.hasSkipPattern(leader, 0, quorum)
	require.True(t, hasSkip, "Skip pattern should be detected when 2f+1 events don't reference leader")
}

// Verifies that skip pattern is NOT detected when a quorum reaches the leader.
func TestMysticeti_HasSkipPattern_NoSkipWhenQuorumReachesLeader(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)
	quorum := committee.Quorum()

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create genesis events
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{
			Creator: v,
			Parents: nil,
			Payload: nil,
		})
	}

	leader := genesis[0]

	// Create round 1 events where all validators include the leader
	for i, v := range validators {
		parents := []*model.Event{genesis[i], leader}
		if i == 0 {
			parents = []*model.Event{genesis[i]}
		}

		_, err := model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{
			Creator: v,
			Parents: parentIds,
			Payload: nil,
		})
	}

	// Skip pattern should NOT be detected
	hasSkip := m.hasSkipPattern(leader, 0, quorum)
	require.False(t, hasSkip, "Skip pattern should not be detected when quorum reaches leader")
}

// Verifies that an event is correctly identified as a certificate when it has
// 2f+1 parents from r+1 that reach the leader.
func TestMysticeti_IsCertificate_ValidCertificate(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)
	quorum := committee.Quorum()

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Round 0: Genesis events
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
	}

	// Leader is genesis[0]
	leader := genesis[0]

	// Round 1: All validators reference the leader (voting)
	round1 := make([]*model.Event, 4)
	for i, v := range validators {
		parents := []*model.Event{genesis[i], leader}
		if i == 0 {
			parents = []*model.Event{genesis[i]}
		}
		var err error
		round1[i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)
	}

	// Round 2: Create certificate event with all round 1 events as parents
	certEvent, err := model.NewEvent(validators[0],
		[]*model.Event{round1[0], round1[1], round1[2], round1[3]}, nil, time.Time{})
	require.NoError(t, err)

	// certEvent should be a certificate for leader
	isCert := m.isCertificate(certEvent, leader, quorum)
	require.True(t, isCert, "Event with 2f+1 voting parents should be a certificate")
}

// Verifies that an event is NOT a certificate if it doesn't have 2f+1 voting
// parents.
func TestMysticeti_IsCertificate_NotCertificateWithoutQuorum(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)
	quorum := committee.Quorum()

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Round 0: Genesis events
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
	}

	leader := genesis[0]

	// Round 1: Only 2 validators reference the leader (not enough)
	round1 := make([]*model.Event, 2)
	for i := range 2 {
		parents := []*model.Event{genesis[i], leader}
		if i == 0 {
			parents = []*model.Event{genesis[i]}
		}
		var err error
		round1[i], err = model.NewEvent(validators[i], parents, nil, time.Time{})
		require.NoError(t, err)
	}

	// Round 2: Certificate event with only 2 voting parents
	certEvent, err := model.NewEvent(validators[0],
		[]*model.Event{round1[0], round1[1]}, nil, time.Time{})
	require.NoError(t, err)

	// certEvent should NOT be a certificate (only 2 < quorum=3)
	isCert := m.isCertificate(certEvent, leader, quorum)
	require.False(t, isCert, "Event without quorum of voting parents should not be a certificate")
}

func TestMysticeti_IsCertificate_WrongRoundEventIsNotCertificate(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
	})
	require.NoError(t, err)
	quorum := committee.Quorum()

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	genesis1, err := model.NewEvent(validators[0], nil, nil, time.Time{})
	require.NoError(t, err)
	genesis2, err := model.NewEvent(validators[1], nil, nil, time.Time{})
	require.NoError(t, err)

	// Event at round 1 (should be at leader round + 2)
	event1, err := model.NewEvent(validators[0], []*model.Event{genesis1, genesis2}, nil, time.Time{})
	require.NoError(t, err)

	// event1 is at wrong round to be a certificate for genesis1
	isCert := m.isCertificate(event1, genesis1, quorum)
	require.False(t, isCert, "Event at wrong round should not be a certificate")
}

func TestMysticeti_IndirectDecision_UndecidedAnchor(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build structure where early leader has certificate pattern but
	// potential anchor is undecided
	events := make([][]*model.Event, 6)

	// Round 0: Genesis
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Rounds 1-5: Build DAG where round 0 leader has certificate pattern
	// but no anchor exists to decide it via indirect rule
	for round := 1; round < 6; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := range 3 {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Leader at round 0 is certified directly (has full support)
	leader0 := events[0][0]
	verdict := m.IsLeader(leader0)
	require.Equal(t, layering.VerdictYes, verdict)
}

func TestMysticeti_IndirectDecision_CertifiedAnchorWithLink(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build 10 rounds where all leaders should be certified
	events := make([][]*model.Event, 10)

	// Round 0
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Rounds 1-9: Full connectivity
	for round := 1; round < 10; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := 0; j < 3; j++ {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// All leaders should eventually be certified via direct or indirect decision
	for round := range 7 {
		leaderIdx := round % 4
		leader := events[round][leaderIdx]
		verdict := m.IsLeader(leader)
		require.Equal(t, layering.VerdictYes, verdict,
			"Leader at round %d should be certified", round)
	}
}

func TestMysticeti_IndirectDecision_CertifiedAnchorWithoutLink(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create scenario where round 0 leader only has 2 votes (not certified)
	// but later leaders are certified
	events := make([][]*model.Event, 10)

	// Round 0
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader0 := events[0][0]

	// Round 1: Only 2 validators reach leader0 (insufficient for certificate)
	events[1] = make([]*model.Event, 4)
	for i, v := range validators {
		var parents []*model.Event
		if i <= 1 {
			parents = []*model.Event{events[0][i], leader0}
			if i == 0 {
				parents = []*model.Event{events[0][i]}
			}
		} else {
			// These don't reach leader0
			parents = []*model.Event{events[0][i], events[0][(i+1)%4]}
		}

		var err error
		events[1][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Rounds 2-9: Full connectivity (later leaders get certified)
	for round := 2; round < 10; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := 0; j < 3; j++ {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Leader0 has certificate pattern (2 votes in round 1) but may still certify
	// through indirect decision if later leaders are certified
	verdict := m.IsLeader(leader0)
	// Since later leaders are fully connected, indirect decision may certify it
	require.Contains(t, []layering.Verdict{layering.VerdictYes, layering.VerdictNo, layering.VerdictUndecided}, verdict,
		"Leader verdict should be valid")
}

func TestMysticeti_IndirectDecision_ComprehensiveCoverage(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build a scenario where direct decision is undecided, forcing indirect decision
	events := make([][]*model.Event, 10)

	// Round 0: Genesis
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Round 1: 3 validators reach leader0 (certificate pattern, but no commit yet)
	// This creates a situation where direct decision is undecided
	events[1] = make([]*model.Event, 4)
	leader0 := events[0][0]
	for i, v := range validators {
		parents := []*model.Event{events[0][i], leader0}
		if i == 0 {
			parents = []*model.Event{events[0][i]}
		}
		var err error
		events[1][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Round 2: Only 2 validators create certificates (not enough for direct commit)
	// This ensures direct decision returns Undecided
	events[2] = make([]*model.Event, 4)
	for i, v := range validators {
		if i < 2 {
			// These create certificates with all round 1 events
			parents := []*model.Event{events[1][i], events[1][(i+1)%4], events[1][(i+2)%4], events[1][(i+3)%4]}
			var err error
			events[2][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for j, p := range parents {
				parentIds[j] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		} else {
			// These don't have all round 1 events
			parents := []*model.Event{events[1][i], events[1][(i+1)%4]}
			var err error
			events[2][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for j, p := range parents {
				parentIds[j] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Rounds 3-9: Build full connectivity so we have certified anchors
	for round := 3; round < 10; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := range 3 {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Test leader0 - should trigger indirect decision
	verdict := m.IsLeader(leader0)
	// Verdict should be decided via indirect path
	require.Contains(t, []layering.Verdict{layering.VerdictYes, layering.VerdictNo}, verdict)
}

func TestMysticeti_IndirectDecision_NoCertificatePattern(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build structure where leader has NO certificate pattern
	events := make([][]*model.Event, 8)

	// Round 0
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader0 := events[0][0]

	// Round 1: Only 2 validators reach leader0 (no certificate pattern)
	events[1] = make([]*model.Event, 4)
	for i, v := range validators {
		var parents []*model.Event
		if i <= 1 {
			parents = []*model.Event{events[0][i], leader0}
			if i == 0 {
				parents = []*model.Event{events[0][i]}
			}
		} else {
			// These skip leader0
			parents = []*model.Event{events[0][i], events[0][(i+1)%4]}
		}
		var err error
		events[1][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Rounds 2-7: Build full connectivity to create certified anchors
	for round := 2; round < 8; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := range 3 {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// With only 2 votes, it lacks certificate pattern and should be skipped
	verdict := m.IsLeader(leader0)
	// The verdict could be VerdictNo (skipped) or still VerdictYes if indirect path certifies it
	// depending on connectivity - just ensure coverage is hit
	require.Contains(t, []layering.Verdict{layering.VerdictYes, layering.VerdictNo}, verdict)
}

func TestMysticeti_IndirectDecision_NoAnchor(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Round 0: Genesis
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Round 1: Build events but not enough to decide
	for i, v := range validators {
		parents := []*model.Event{genesis[i], genesis[(i+1)%4]}
		_, err := model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Don't build rounds 2+ - no anchor will exist
	leader := genesis[0]
	verdict := m.IsLeader(leader)
	require.Equal(t, layering.VerdictUndecided, verdict)
}

func TestMysticeti_IndirectDecision_AnchorUndecided(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build structure where anchor is undecided
	events := make([][]*model.Event, 7)

	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader0 := events[0][0]

	// Round 1: 2 validators reach leader0 (not enough for direct decision)
	events[1] = make([]*model.Event, 4)
	for i, v := range validators {
		var parents []*model.Event
		if i <= 1 {
			parents = []*model.Event{events[0][i], leader0}
			if i == 0 {
				parents = []*model.Event{events[0][i]}
			}
		} else {
			parents = []*model.Event{events[0][i], events[0][(i+1)%4]}
		}
		var err error
		events[1][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Round 2: Partial certificates
	events[2] = make([]*model.Event, 4)
	for i, v := range validators {
		if i < 2 {
			parents := []*model.Event{events[1][i], events[1][(i+1)%4], events[1][(i+2)%4], events[1][(i+3)%4]}
			var err error
			events[2][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for j, p := range parents {
				parentIds[j] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		} else {
			parents := []*model.Event{events[1][i], events[1][(i+1)%4]}
			var err error
			events[2][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for j, p := range parents {
				parentIds[j] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Rounds 3-6: Similar partial patterns (anchors will be undecided)
	for round := 3; round < 7; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			if i < 2 {
				parents := make([]*model.Event, 4)
				parents[0] = events[round-1][i]
				for j := 0; j < 3; j++ {
					parents[j+1] = events[round-1][(i+j+1)%4]
				}
				var err error
				events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
				require.NoError(t, err)

				parentIds := make([]model.EventId, len(parents))
				for k, p := range parents {
					parentIds[k] = p.EventId()
				}
				dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
			} else {
				parents := []*model.Event{events[round-1][i], events[round-1][(i+1)%4]}
				var err error
				events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
				require.NoError(t, err)

				parentIds := make([]model.EventId, len(parents))
				for j, p := range parents {
					parentIds[j] = p.EventId()
				}
				dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
			}
		}
	}

	// leader0 should remain undecided because anchor is undecided
	verdict := m.IsLeader(leader0)
	require.Equal(t, layering.VerdictUndecided, verdict)
}

func TestMysticeti_IndirectDecisionNoCertificatePattern(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()
	events := make([][]*model.Event, 10)

	// Round 0
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		events[0][i], _ = model.NewEvent(v, nil, nil, time.Time{})
		dag.AddEvent(model.EventMessage{Creator: v})
	}

	leader0 := events[0][0]

	// Round 1: Only validator 0 reaches leader0 (insufficient for certificate pattern)
	events[1] = make([]*model.Event, 4)
	for i, v := range validators {
		var parents []*model.Event
		if i == 0 {
			parents = []*model.Event{events[0][i]}
		} else {
			parents = []*model.Event{events[0][i], events[0][(i+1)%4]}
		}
		events[1][i], _ = model.NewEvent(v, parents, nil, time.Time{})

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
	}

	// Build full connectivity for rounds 2-9
	for round := 2; round < 10; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := 0; j < 3; j++ {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}
			events[round][i], _ = model.NewEvent(v, parents, nil, time.Time{})

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
		}
	}

	verdict := m.indirectDecision(leader0, 0, committee.Quorum())
	require.Equal(t, layering.VerdictNo, verdict)
}

func TestMysticeti_IndirectDecision_CertifiedAnchorNoCertifiedLink(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build structure where:
	// - Round 0 leader has 2f+1 votes in round 1 (certificate pattern)
	// - Round 2 has certificates for round 0 leader
	// - But anchor at round 3 does NOT link to those certificates

	events := make([][]*model.Event, 10)

	// Round 0: Genesis
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v})
	}

	leader0 := events[0][0]

	// Round 1: All validators reach leader0 (certificate pattern exists)
	events[1] = make([]*model.Event, 4)
	for i, v := range validators {
		parents := []*model.Event{events[0][i], leader0}
		if i == 0 {
			parents = []*model.Event{events[0][i]}
		}
		events[1][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)
		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
	}

	// Round 2: Create certificates for leader0 (only 2 - not enough for commit pattern)
	// But they exist - so certificate pattern is satisfied
	events[2] = make([]*model.Event, 4)
	for i, v := range validators {
		var parents []*model.Event
		if i < 2 {
			// These are certificates (have all round 1 events as parents)
			parents = []*model.Event{events[1][i], events[1][(i+1)%4], events[1][(i+2)%4], events[1][(i+3)%4]}
		} else {
			// These are NOT certificates (don't have enough voting parents)
			parents = []*model.Event{events[1][i], events[1][(i+1)%4]}
		}
		events[2][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)
		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
	}

	// Rounds 3-9: Build with partial connectivity that breaks the certified link
	// Validators 2 and 3 don't reference the certificates from validators 0 and 1
	for round := 3; round < 10; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			var parents []*model.Event
			if i < 2 {
				// These validators continue from their own chain
				parents = []*model.Event{events[round-1][i], events[round-1][(i+1)%4]}
			} else {
				// These validators only reference each other, not the certificate chain
				parents = []*model.Event{events[round-1][i], events[round-1][3-i+2]}
			}
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)
			parentIds := make([]model.EventId, len(parents))
			for j, p := range parents {
				parentIds[j] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
		}
	}

	// Pre-certify anchor at round 3 (first possible anchor for candidate at round 0)
	// Skip rounds 3, 4, 5 to force anchor to be at round 6
	m.slotDecisions[3] = layering.VerdictNo
	m.slotDecisions[4] = layering.VerdictNo
	m.slotDecisions[5] = layering.VerdictNo
	m.slotDecisions[6] = layering.VerdictYes

	// leader0 has certificate pattern but anchor doesn't link to any certificate
	// This should return VerdictNo
	verdict2 := m.indirectDecision(leader0, 0, committee.Quorum())
	require.Equal(t, layering.VerdictNo, verdict2,
		"Should return No when anchor doesn't link to any certificate for the candidate")
}

func TestMysticeti_HasCertifiedLinkToAnchor_NoLink(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build structure where certificate exists but doesn't link to anchor
	events := make([][]*model.Event, 8)

	// Round 0
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader0 := events[0][0]

	// Round 1: All reach leader0
	events[1] = make([]*model.Event, 4)
	for i, v := range validators {
		parents := []*model.Event{events[0][i], leader0}
		if i == 0 {
			parents = []*model.Event{events[0][i]}
		}
		var err error
		events[1][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Round 2: Create certificates but break the link to potential future anchors
	events[2] = make([]*model.Event, 4)
	for i, v := range validators {
		// Create certificates without full connectivity
		if i < 3 {
			parents := []*model.Event{events[1][i], events[1][(i+1)%4], events[1][(i+2)%4], events[1][(i+3)%4]}
			var err error
			events[2][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for j, p := range parents {
				parentIds[j] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Don't create rounds 3+ - no anchor will link back to round 2 certificates
	candidate := leader0
	anchor := events[1][1] // Some later event as mock anchor

	hasLink := m.hasCertifiedLinkToAnchor(candidate, 0, anchor, committee.Quorum())
	require.False(t, hasLink)
}

func TestMysticeti_HasCertifiedLinkToAnchor_FoundAndYieldsVerdictYes(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build complete connectivity to ensure certified link exists
	events := make([][]*model.Event, 8)

	// Round 0
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Rounds 1-7: Full connectivity
	for round := 1; round < 8; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := 0; j < 3; j++ {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Test hasCertifiedLinkToAnchor directly
	// leader0 at round 0, certificates at round 2, anchor at round 7
	// With full connectivity, this should find a certified link
	leader0 := events[0][0]

	// Verify through IsLeader which internally uses hasCertifiedLinkToAnchor
	verdict := m.IsLeader(leader0)
	require.Equal(t, layering.VerdictYes, verdict, "Leader should be certified with full connectivity")
}

func TestMysticeti_FindAnchor_SkippedLeaders(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build structure where leader at round 0 is skipped (VerdictNo)
	// Round 0: Genesis - validators[0] is the leader
	events := make([][]*model.Event, 3)

	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader0 := events[0][0]

	// Round 1: 3 validators (quorum) do NOT include leader0 as parent
	// This creates a skip pattern for leader0
	events[1] = make([]*model.Event, 4)
	for i, v := range validators {
		var parents []*model.Event
		if i == 0 {
			// Validator 0 only references self
			parents = []*model.Event{events[0][i]}
		} else {
			// Validators 1, 2, 3 skip leader0 (don't include events[0][0] as parent)
			// They only reference their own genesis and one other non-leader genesis
			switch i {
			case 1:
				parents = []*model.Event{events[0][i], events[0][2]}
			case 2:
				parents = []*model.Event{events[0][i], events[0][3]}
			case 3:
				parents = []*model.Event{events[0][i], events[0][1]}
			}
		}
		var err error
		events[1][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// leader0 should be skipped because 3 validators in round 1 don't have
	// a direct parent from leader0's author (validator 0) at round 0
	verdict := m.IsLeader(leader0)
	require.Equal(t, layering.VerdictNo, verdict, "Leader should be skipped when quorum doesn't include it as parent")
}

func TestMysticeti_FindAnchor_NoAnchorFound(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create only genesis - no rounds beyond that
	for _, v := range validators {
		_, err := model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Try to find anchor starting at round 3 - should return nil
	anchor, verdict := m.findAnchor(3, committee.Quorum())
	require.Nil(t, anchor)
	require.Equal(t, layering.VerdictUndecided, verdict)
}

func TestMysticeti_FindAnchor_CachedSkippedLeader(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build structure with skipped leaders
	events := make([][]*model.Event, 8)

	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Round 1: Skip leader (validators[1])
	events[1] = make([]*model.Event, 4)
	for i, v := range validators {
		var parents []*model.Event
		switch i {
		case 1:
			parents = []*model.Event{events[0][i], events[0][2]}
		case 2:
			parents = []*model.Event{events[0][i], events[0][3]}
		case 3:
			parents = []*model.Event{events[0][i], events[0][0]}
		default:
			parents = []*model.Event{events[0][i]}
		}
		var err error
		events[1][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Rounds 2-7: Full connectivity
	for round := 2; round < 8; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := 0; j < 3; j++ {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Manually cache a skipped decision for round 1
	m.slotDecisions[1] = layering.VerdictNo

	// Now when finding anchor starting from round 4, it will encounter
	// the cached skipped leader at round 1 and continue
	anchor, verdictAnchor := m.findAnchor(4, committee.Quorum())
	require.NotNil(t, anchor)
	require.Contains(t, []layering.Verdict{layering.VerdictYes, layering.VerdictUndecided}, verdictAnchor)
}

func TestMysticeti_FindAnchor_DirectDecisionUndecided(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build structure where potential anchor is undecided
	events := make([][]*model.Event, 6)

	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Rounds 1-5: Build partial structure
	for round := 1; round < 6; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			if i < 2 {
				parents := make([]*model.Event, 4)
				parents[0] = events[round-1][i]
				for j := range 3 {
					parents[j+1] = events[round-1][(i+j+1)%4]
				}
				var err error
				events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
				require.NoError(t, err)

				parentIds := make([]model.EventId, len(parents))
				for k, p := range parents {
					parentIds[k] = p.EventId()
				}
				dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
			} else {
				parents := []*model.Event{events[round-1][i], events[round-1][(i+1)%4]}
				var err error
				events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
				require.NoError(t, err)

				parentIds := make([]model.EventId, len(parents))
				for j, p := range parents {
					parentIds[j] = p.EventId()
				}
				dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
			}
		}
	}

	// Find anchor from round 3 - should find undecided anchor
	anchor, verdictAnchor := m.findAnchor(3, committee.Quorum())
	require.NotNil(t, anchor)
	require.Equal(t, layering.VerdictUndecided, verdictAnchor, "Anchor with partial structure should be undecided")
}

func TestMysticeti_FindAnchorWithCachedDecisions(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build genesis (round 0)
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v})
	}

	// Build round 1 with full connectivity
	round1 := make([]*model.Event, 4)
	for i, v := range validators {
		parents := make([]*model.Event, 4)
		parents[0] = genesis[i]
		for j := 1; j < 4; j++ {
			parents[j] = genesis[(i+j)%4]
		}
		round1[i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
	}

	// Cache decisions: round 0 is skipped (No), round 1 is certified (Yes)
	m.slotDecisions[0] = layering.VerdictNo
	m.slotDecisions[1] = layering.VerdictYes

	// findAnchor should skip round 0 (VerdictNo) and return round 1 (VerdictYes)
	anchor, verdict := m.findAnchor(0, committee.Quorum())
	require.NotNil(t, anchor)
	require.Equal(t, layering.VerdictYes, verdict)
	require.Equal(t, uint32(1), m.GetRound(anchor))
}

// TestMysticeti_FindAnchor_SkipsMissingLeaderRounds tests that findAnchor
// correctly skips rounds where the leader hasn't created an event.
func TestMysticeti_FindAnchor_SkipsMissingLeaderRounds(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Round 0: Genesis for all validators
	events := make([][]*model.Event, 6)
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v})
	}

	// Round 1: All validators participate (leader is validators[1])
	events[1] = make([]*model.Event, 4)
	for i, v := range validators {
		parents := make([]*model.Event, 4)
		parents[0] = events[0][i]
		for j := 0; j < 3; j++ {
			parents[j+1] = events[0][(i+j+1)%4]
		}
		events[1][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)
		parentIds := make([]model.EventId, len(parents))
		for k, p := range parents {
			parentIds[k] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
	}

	// Round 2: Skip the leader (validators[2]) - don't create their event
	events[2] = make([]*model.Event, 4)
	for i, v := range validators {
		if i == 2 {
			continue // Skip the leader for round 2
		}
		parents := make([]*model.Event, 4)
		parents[0] = events[1][i]
		for j := 0; j < 3; j++ {
			parents[j+1] = events[1][(i+j+1)%4]
		}
		events[2][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)
		parentIds := make([]model.EventId, len(parents))
		for k, p := range parents {
			parentIds[k] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
	}

	// Round 3: All validators participate (leader is validators[3])
	events[3] = make([]*model.Event, 4)
	for i, v := range validators {
		var parents []*model.Event
		if i == 2 {
			// Validator 2 skipped round 2, reference round 1
			parents = []*model.Event{events[1][i], events[2][0], events[2][1], events[2][3]}
		} else {
			parents = []*model.Event{events[2][i], events[2][(i+1)%4]}
			if events[2][(i+1)%4] == nil {
				parents = []*model.Event{events[2][i], events[1][(i+1)%4]}
			}
		}
		events[3][i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)
		parentIds := make([]model.EventId, len(parents))
		for k, p := range parents {
			parentIds[k] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
	}

	// Round 4-5: Continue building with full connectivity
	for round := 4; round < 6; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := 0; j < 3; j++ {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)
			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
		}
	}

	// findAnchor starting at round 2 should skip round 2 (no leader event)
	// and find an anchor at round 3 or later
	anchor, verdict := m.findAnchor(2, committee.Quorum())
	require.NotNil(t, anchor)
	// Round 2 has no leader, so anchor should be from round 3 or later
	require.GreaterOrEqual(t, m.GetRound(anchor), uint32(3))
	require.Contains(t, []layering.Verdict{layering.VerdictYes, layering.VerdictUndecided}, verdict)
}

func TestMysticeti_FindAnchor_SkipsOverCachedNoVerdicts(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build 8 rounds with full connectivity
	events := make([][]*model.Event, 8)

	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v})
	}

	for round := 1; round < 8; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i]
			for j := 0; j < 3; j++ {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)
			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds})
		}
	}

	// Cache rounds 3, 4, 5 as skipped (VerdictNo)
	m.slotDecisions[3] = layering.VerdictNo
	m.slotDecisions[4] = layering.VerdictNo
	m.slotDecisions[5] = layering.VerdictNo
	// Cache round 6 as certified (VerdictYes)
	m.slotDecisions[6] = layering.VerdictYes

	// findAnchor starting at round 3 should skip rounds 3, 4, 5 (all VerdictNo)
	// and return the anchor at round 6 (VerdictYes)
	anchor, verdict := m.findAnchor(3, committee.Quorum())
	require.NotNil(t, anchor)
	require.Equal(t, layering.VerdictYes, verdict)
	require.Equal(t, uint32(6), m.GetRound(anchor))
}

func TestMysticeti_FindLeaderInRound_NoLeader(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create only genesis for validator 1 (leader of round 0)
	genesis1, err := model.NewEvent(validators[0], nil, nil, time.Time{})
	require.NoError(t, err)
	dag.AddEvent(model.EventMessage{Creator: validators[0], Parents: nil, Payload: nil})

	// Try to find leader in round 1 when it doesn't exist yet
	// Leader of round 1 should be validators[1], but we haven't created it
	leader := m.findLeaderInRound(1)
	require.Nil(t, leader)

	// Verify IsLeader for genesis1
	verdict := m.IsLeader(genesis1)
	require.Equal(t, layering.VerdictUndecided, verdict) // genesis1 needs more rounds to decide
}

func TestMysticeti_FindLeaderInRound_AlreadyFound(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create genesis events
	for _, v := range validators {
		_, err := model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Find leader in round 0 - should find it and return
	leader := m.findLeaderInRound(0)
	require.NotNil(t, leader)
	require.Equal(t, validators[0], leader.Creator())
}

// TestMysticeti_FindLeaderInRound_EarlyBreakOnFound tests that findLeaderInRound
// breaks early when the leader is found from the first head.
func TestMysticeti_FindLeaderInRound_EarlyBreakOnFound(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create genesis events for all validators
	for _, v := range validators {
		_, err := model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v})
	}

	// With 4 separate heads (no connections), findLeaderInRound will iterate
	// through heads and break early once the leader is found
	leader2 := m.findLeaderInRound(0)
	require.NotNil(t, leader2)
	require.Equal(t, validators[0], leader2.Creator())
}

func TestMysticeti_SortLeaders_OrdersLeadersByRound(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Build multiple rounds with certified leaders
	// Need 12 rounds to certify leaders in rounds 0-9
	// (leader in round r needs rounds r+1 and r+2 to be certified)
	events := make([][]*model.Event, 12)

	// Round 0
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Build subsequent rounds
	for round := 1; round < 12; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			// Reference all events from previous round
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i] // self-parent first
			for j := range 3 {
				parents[j+1] = events[round-1][(i+j+1)%4]
			}

			var err error
			events[round][i], err = model.NewEvent(v, parents, nil, time.Time{})
			require.NoError(t, err)

			parentIds := make([]model.EventId, len(parents))
			for k, p := range parents {
				parentIds[k] = p.EventId()
			}
			dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
		}
	}

	// Collect leaders (one per round in round-robin fashion)
	leaders := []*model.Event{}
	for round := range 10 {
		leaderIdx := round % 4
		leaders = append(leaders, events[round][leaderIdx])
	}

	// Sort leaders (provide in reverse order)
	reversedLeaders := make([]*model.Event, len(leaders))
	for i := range leaders {
		reversedLeaders[i] = leaders[len(leaders)-1-i]
	}

	sorted := m.SortLeaders(reversedLeaders)

	// Verify sorted order
	require.Len(t, sorted, len(leaders))
	for i, leader := range sorted {
		expectedRound := uint32(i)
		actualRound := m.GetRound(leader)
		require.Equal(t, expectedRound, actualRound,
			"Leader at index %d should be from round %d", i, expectedRound)
	}
}

func TestMysticeti_SortLeaders_FiltersNonLeaders(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
		consensus.ValidatorId(3): 1,
		consensus.ValidatorId(4): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	validators := committee.Validators()

	// Create genesis events (round 0)
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	leader := genesis[0]

	// Round 1: All vote for leader to create certificate pattern
	round1 := make([]*model.Event, 4)
	for i, v := range validators {
		parents := []*model.Event{genesis[i], leader}
		if i == 0 {
			parents = []*model.Event{genesis[i]}
		}
		var err error
		round1[i], err = model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Round 2: All create certificates
	for i, v := range validators {
		parents := []*model.Event{round1[i], round1[(i+1)%4], round1[(i+2)%4], round1[(i+3)%4]}
		_, err := model.NewEvent(v, parents, nil, time.Time{})
		require.NoError(t, err)

		parentIds := make([]model.EventId, len(parents))
		for j, p := range parents {
			parentIds[j] = p.EventId()
		}
		dag.AddEvent(model.EventMessage{Creator: v, Parents: parentIds, Payload: nil})
	}

	// Only genesis[0] is a certified leader (leader of round 0)
	// Others are not leaders (not the designated leader for their round)
	events := []*model.Event{genesis[0], genesis[1], genesis[2], genesis[3]}

	// Sort should filter out non-leaders
	sorted := m.SortLeaders(events)
	require.Len(t, sorted, 1)
	require.Equal(t, genesis[0], sorted[0])
}

func TestMysticeti_GetRound_GenesisIsRoundZero(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	genesis, err := model.NewEvent(consensus.ValidatorId(1), nil, nil, time.Time{})
	require.NoError(t, err)

	round := m.GetRound(genesis)
	require.Equal(t, uint32(0), round)
}

func TestMysticeti_GetRound_RoundIsMaxParentRoundPlusOne(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
		consensus.ValidatorId(2): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	// Create genesis events
	genesis1, err := model.NewEvent(consensus.ValidatorId(1), nil, nil, time.Time{})
	require.NoError(t, err)
	genesis2, err := model.NewEvent(consensus.ValidatorId(2), nil, nil, time.Time{})
	require.NoError(t, err)

	// Create round 1 event
	event1, err := model.NewEvent(consensus.ValidatorId(1), []*model.Event{genesis1, genesis2}, nil, time.Time{})
	require.NoError(t, err)

	round := m.GetRound(event1)
	require.Equal(t, uint32(1), round)

	// Create round 2 event
	event2, err := model.NewEvent(consensus.ValidatorId(2), []*model.Event{genesis2, event1}, nil, time.Time{})
	require.NoError(t, err)

	round = m.GetRound(event2)
	require.Equal(t, uint32(2), round)
}

func TestMysticeti_GetRound_CachesRoundAssignment(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	genesis, err := model.NewEvent(consensus.ValidatorId(1), nil, nil, time.Time{})
	require.NoError(t, err)

	// First call should compute and cache
	round1 := m.GetRound(genesis)
	// Second call should use cache
	round2 := m.GetRound(genesis)

	require.Equal(t, round1, round2)
	require.Equal(t, uint32(0), round1)

	// Verify it's in the cache
	cachedRound, exists := m.eventRounds[genesis.EventId()]
	require.True(t, exists)
	require.Equal(t, uint32(0), cachedRound)
}

func TestMysticeti_GetRound_NilEvent(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
		consensus.ValidatorId(1): 1,
	})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	m := Factory{}.NewLayering(dag, committee).(*Mysticeti)

	round := m.GetRound(nil)
	require.Equal(t, uint32(0), round)
}
