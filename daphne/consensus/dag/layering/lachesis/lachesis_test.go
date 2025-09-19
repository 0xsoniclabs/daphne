package lachesis

import (
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestLachesis(t *testing.T) {
	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{
		1: 1,
		2: 1,
	})

	require.NoError(t, err)

	lachesis := (&Factory{}).NewLayering(committee).(*Lachesis)
	dag := model.NewDag()

	event_1_1, _ := model.NewEvent(1, nil, nil)
	event_2_1, _ := model.NewEvent(2, nil, nil)

	event_1_2, _ := model.NewEvent(1, []*model.Event{event_1_1, event_2_1}, nil)

	require.False(t, lachesis.stronglyReaches(event_1_2, event_1_1))
	require.True(t, lachesis.stronglyReaches(event_1_2, event_2_1))

	require.True(t, lachesis.IsCandidate(event_1_1))
	require.True(t, lachesis.IsCandidate(event_2_1))
	require.False(t, lachesis.IsCandidate(event_1_2))

	event_2_2, _ := model.NewEvent(2, []*model.Event{event_2_1, event_1_2}, nil)
	event_1_3, _ := model.NewEvent(1, []*model.Event{event_1_2, event_2_2}, nil)

	require.True(t, lachesis.IsCandidate(event_2_2))
	require.True(t, lachesis.IsCandidate(event_1_3))

	event_1_4, _ := model.NewEvent(1, []*model.Event{event_1_3, event_2_2}, nil)
	require.False(t, lachesis.IsCandidate(event_1_4))

	event_2_3, _ := model.NewEvent(2, []*model.Event{event_2_2, event_1_4}, nil)
	require.True(t, lachesis.IsCandidate(event_2_3))

	event_1_1 = dag.AddEvent(event_1_1.ToEventMessage())[0]
	event_2_1 = dag.AddEvent(event_2_1.ToEventMessage())[0]
	event_1_2 = dag.AddEvent(event_1_2.ToEventMessage())[0]
	event_2_2 = dag.AddEvent(event_2_2.ToEventMessage())[0]
	event_1_3 = dag.AddEvent(event_1_3.ToEventMessage())[0]
	event_1_4 = dag.AddEvent(event_1_4.ToEventMessage())[0]
	event_2_3 = dag.AddEvent(event_2_3.ToEventMessage())[0]

	require.Equal(t, 3, lachesis.eventFrame(event_2_3))
	require.True(t, lachesis.IsCandidate(event_2_3))
	// require.Equal(t, layering.VerdictUndecided, lachesis.IsLeader(dag, event_2_3))
	// require.Equal(t, layering.VerdictNo, lachesis.IsLeader(dag, event_1_4))

	require.Equal(t, layering.VerdictYes, lachesis.IsLeader(dag, event_2_1))
	require.Equal(t, layering.VerdictNo, lachesis.IsLeader(dag, event_1_1))
}
