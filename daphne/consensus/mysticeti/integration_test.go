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
	"github.com/stretchr/testify/require"
)

func TestMysticeti_Integration_FourValidatorsCommitSequence(t *testing.T) {
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

	// Build 6 rounds of events
	events := make([][]*model.Event, 6)

	// Round 0: Genesis
	events[0] = make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		events[0][i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	// Rounds 1-5: Each validator references all from previous round
	for round := 1; round < 6; round++ {
		events[round] = make([]*model.Event, 4)
		for i, v := range validators {
			parents := make([]*model.Event, 4)
			parents[0] = events[round-1][i] // self-parent
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

	// Verify leaders are certified for rounds 0-3 (need 3 rounds to certify)
	for round := 0; round <= 3; round++ {
		leaderIdx := round % 4
		leader := events[round][leaderIdx]

		verdict := m.IsLeader(leader)
		require.Equal(t, layering.VerdictYes, verdict,
			"Leader of round %d should be certified", round)
	}

	// Collect and sort all leaders
	allLeaders := []*model.Event{}
	for round := range 6 {
		leaderIdx := round % 4
		allLeaders = append(allLeaders, events[round][leaderIdx])
	}

	sorted := m.SortLeaders(allLeaders)
	require.GreaterOrEqual(t, len(sorted), 4, "At least first 4 leaders should be certified")

	// Verify sorted order
	for i := range sorted {
		expectedRound := uint32(i)
		actualRound := m.GetRound(sorted[i])
		require.Equal(t, expectedRound, actualRound)
	}
}

func TestMysticeti_Integration_CrashFaultTolerance(t *testing.T) {
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

	// Round 0: Genesis - validators[0] is the leader
	genesis := make([]*model.Event, 4)
	for i, v := range validators {
		var err error
		genesis[i], err = model.NewEvent(v, nil, nil, time.Time{})
		require.NoError(t, err)
		dag.AddEvent(model.EventMessage{Creator: v, Parents: nil, Payload: nil})
	}

	crashedLeader := genesis[0]

	// Round 1: Simulate crash - only 3 validators participate and skip the leader
	// This creates a skip pattern
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

	// Crashed leader should be skipped
	verdict := m.IsLeader(crashedLeader)
	require.Equal(t, layering.VerdictNo, verdict,
		"Crashed validator's leader event should be skipped")
}
