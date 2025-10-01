package lachesis

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
)

func TestLachesis_IsALayeringImplementation(t *testing.T) {
	var _ layering.Layering = &Lachesis{}
}

func TestLachesis_IsCandidate_ReturnsFalseForIllegalEvents(t *testing.T) {
	require := require.New(t)

	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{1: 1})
	require.NoError(err)

	lachesis := (&Factory{}).NewLayering(committee)
	require.False(lachesis.IsCandidate(nil))

	event, err := model.NewEvent(2, nil, nil)
	require.NoError(err)
	require.False(lachesis.IsCandidate(event))
}

func TestLachesis_IsCandidate_TrueForFirstInFrameCandidate(t *testing.T) {
	require := require.New(t)

	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{1: 1, 2: 1})
	require.NoError(err)

	lachesis := (&Factory{}).NewLayering(committee)

	// e_#creatorid_#seq
	// c - candidate

	// e_1_3(c)═══╬
	// ║          ║
	// ╬════════e_2_2(c)
	// ║          ║
	// e_1_2══════╬
	// ║          ║
	// e_1_1(c) e_2_1(c)

	e_1_1, err := model.NewEvent(1, nil, nil)
	require.NoError(err)
	e_2_1, err := model.NewEvent(2, nil, nil)
	require.NoError(err)
	e_1_2, err := model.NewEvent(1, []*model.Event{e_1_1, e_2_1}, nil)
	require.NoError(err)

	require.True(lachesis.IsCandidate(e_1_1))
	require.True(lachesis.IsCandidate(e_2_1))
	require.False(lachesis.IsCandidate(e_1_2))

	e_2_2, err := model.NewEvent(2, []*model.Event{e_2_1, e_1_2}, nil)
	require.NoError(err)
	e_1_3, err := model.NewEvent(1, []*model.Event{e_1_2, e_2_2}, nil)
	require.NoError(err)

	require.True(lachesis.IsCandidate(e_2_2))
	require.True(lachesis.IsCandidate(e_1_3))
}

func TestLachesis_IsLeader_ElectsLeadersSequentiallyByFrames(t *testing.T) {
	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{1: 2, 2: 1})
	require.NoError(t, err)

	lachesis := (&Factory{}).NewLayering(committee)
	dag := model.NewDag()

	// e_#creatorid_#seq
	// c - candidate
	// l - leader

	// e_1_3(c)═══════╬
	// ║              ║
	// ╬════════════e_2_2(c)
	// ║              ║
	// e_1_2══════════╬
	// ║              ║
	// e_1_1(c)     e_2_1(c)

	e_1_1 := createEventAndAddToDag(t, dag, 1, nil)
	e_2_1 := createEventAndAddToDag(t, dag, 2, nil)
	e_1_2 := createEventAndAddToDag(t, dag, 1, []*model.Event{e_1_1, e_2_1})
	e_2_2 := createEventAndAddToDag(t, dag, 2, []*model.Event{e_2_1, e_1_2})
	e_1_3 := createEventAndAddToDag(t, dag, 1, []*model.Event{e_1_2, e_2_2})
	t.Run("Two frames of candidates, no aggregating voters", func(t *testing.T) {
		// All candidates remain undecided as no voters that can aggregate are
		// available in the DAG.
		require.Equal(t, layering.VerdictUndecided, lachesis.IsLeader(dag, e_1_1))
		require.Equal(t, layering.VerdictUndecided, lachesis.IsLeader(dag, e_2_1))
		require.Equal(t, layering.VerdictUndecided, lachesis.IsLeader(dag, e_2_2))
		require.Equal(t, layering.VerdictUndecided, lachesis.IsLeader(dag, e_1_3))
	})

	// ╬════════════e_2_3(c)
	// ║              ║
	// e_1_4══════════╬
	// ║              ║
	// e_1_3(c)═══════╬
	// ║              ║
	// ╬════════════e_2_2(c)
	// ║              ║
	// e_1_2══════════╬
	// ║              ║
	// e_1_1(c,l)   e_2_1(c)

	e_1_4 := createEventAndAddToDag(t, dag, 1, []*model.Event{e_1_3, e_2_2})
	e_2_3 := createEventAndAddToDag(t, dag, 2, []*model.Event{e_2_2, e_1_4})

	t.Run("Third frame candidate aggregates votes and elects frame 1", func(t *testing.T) {
		require.Equal(t, layering.VerdictUndecided, lachesis.IsLeader(dag, e_2_3))
		// e_1_4 doesn't strongly reach e_1_3, and can't gather a quorum to become a candidate.
		require.Equal(t, layering.VerdictNo, lachesis.IsLeader(dag, e_1_4))
		// e1_1_1 is elected leader by e_2_3 aggregating votes from e_3_1 and e_2_2
		require.Equal(t, layering.VerdictYes, lachesis.IsLeader(dag, e_1_1))
		// and e_2_1 is ruled out.
		require.Equal(t, layering.VerdictNo, lachesis.IsLeader(dag, e_2_1))
	})
}

func TestLachesis_IsLeader_RejectsHighestPriorityCandidate(t *testing.T) {
	require := require.New(t)

	const numCreators = 4
	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{0: 1, 1: 1, 2: 1, 3: 1})
	require.NoError(err)

	lachesis := &Lachesis{
		committee:            committee,
		frameCache:           map[model.EventId]int{},
		stronglyReachesCache: map[eventHashPair]bool{},
	}
	dag := model.NewDag()

	// layers[frame-1][CreatorId]
	layers := make([][]*model.Event, 1)
	// Genesis events (Frame-1 Layer of candidates).
	for i := range numCreators {
		layers[0] = append(layers[0], createEventAndAddToDag(t, dag, model.CreatorId(i), nil))
	}

	addFrameCandidates := func(halfMeshVotes bool) {
		frameIdx := len(layers)
		layers = append(layers, []*model.Event{})
		// Frame-2 Layer of candidates.
		for creatorId := range numCreators {
			parents := slices.Clone(layers[frameIdx-1])
			// Move own creator event to the front.
			parents[0], parents[creatorId] = parents[creatorId], parents[0]
			// We want the creator 0 to be ruled out as a leader, so we remove
			// all of its strongly reaching votes, except from its own creator.
			if halfMeshVotes && creatorId != 0 {
				parents = slices.DeleteFunc(parents, func(e *model.Event) bool { return e.Creator() == model.CreatorId(0) })
			}
			event := createEventAndAddToDag(t, dag, model.CreatorId(creatorId), parents)
			// We are simulating candidate status by priming the stronglyReachesCache.
			for _, parent := range parents {
				lachesis.stronglyReachesCache[eventHashPair{event.EventId(), parent.EventId()}] = true
			}
			require.True(lachesis.IsCandidate(event))
			layers[frameIdx] = append(layers[frameIdx], event)
		}
	}

	// Frame 2 candidates - quorum of Frame 2 candidates can't strongly reach Frame
	// 1 Creator 0 genesis candidate.
	addFrameCandidates(true)
	// Every Frame-1 candidate should be Undecided as no aggregating voters are present.
	for _, candidate := range layers[0] {
		require.Equal(layering.VerdictUndecided, lachesis.IsLeader(dag, candidate))
	}

	// Frame 3 candidates - all candidates strongly reach all Frame 2 candidates.
	addFrameCandidates(false)
	// The Creator 0 candidate should be ruled out as a leader and Creator 1
	// should be elected as it has the next highest priority.
	require.Equal(layering.VerdictNo, lachesis.IsLeader(dag, layers[0][0]))
	require.Equal(layering.VerdictYes, lachesis.IsLeader(dag, layers[0][1]))
}

func TestLachesis_IsLeader_FrameElectionDelayedByLackOfQuorum(t *testing.T) {
	require := require.New(t)

	const numCreators = 4
	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{0: 1, 1: 1, 2: 1, 3: 1})
	require.NoError(err)

	lachesis := &Lachesis{
		committee:            committee,
		frameCache:           map[model.EventId]int{},
		stronglyReachesCache: map[eventHashPair]bool{},
	}
	dag := model.NewDag()

	// layers[frame-1][CreatorId]
	layers := make([][]*model.Event, 1)
	// Genesis events (Frame-1 Layer of candidates).
	for i := range numCreators {
		layers[0] = append(layers[0], createEventAndAddToDag(t, dag, model.CreatorId(i), nil))
	}

	addFrameCandidates := func(halfMeshVotes bool) {
		frameIdx := len(layers)
		layers = append(layers, []*model.Event{})
		// Frame-2 Layer of candidates.
		for creatorId := range numCreators {
			parents := slices.Clone(layers[frameIdx-1])
			// Move own creator event to the front.
			parents[0], parents[creatorId] = parents[creatorId], parents[0]
			// We want the creator 0 genesis candidate to receive only 50% of the votes
			// from 'frame+1' when halfMeshVotes is true, so it can't be instantly
			// elected as leader when `frame+2` candidates are aggregating votes.
			// This delays the election and forces all elections as Undecided.
			// To this end, we remove votes for creator-0 'frame' candidate from creators
			// 1 and 3.
			if halfMeshVotes && creatorId%2 == 1 {
				parents = slices.DeleteFunc(parents, func(e *model.Event) bool { return e.Creator() == model.CreatorId(0) })
			}
			event := createEventAndAddToDag(t, dag, model.CreatorId(creatorId), parents)
			// We are simulating candidate status by priming the stronglyReachesCache.
			for _, parent := range parents {
				lachesis.stronglyReachesCache[eventHashPair{event.EventId(), parent.EventId()}] = true
			}
			require.True(lachesis.IsCandidate(event))
			layers[frameIdx] = append(layers[frameIdx], event)
		}
	}

	// Frame 2 candidates - half of candidates strongly reach creator 0 genesis candidate.
	addFrameCandidates(true)
	// Every Frame-1 candidate should be Undecided as no aggregating voters are present.
	for _, candidate := range layers[0] {
		require.Equal(layering.VerdictUndecided, lachesis.IsLeader(dag, candidate))
	}

	// Frame 3 candidates - half of candidates strongly reach creator 0 frame 1 candidate.
	// This will delay the election of frame 1 candidates as well.
	addFrameCandidates(true)
	// Every candidate should be Undecided as full quorum for Creator 0 (highest priority),
	// cannot be reached due to the missing votes from frame 2.
	for _, candidate := range layers[0] {
		require.Equal(layering.VerdictUndecided, lachesis.IsLeader(dag, candidate))
	}

	// Frame 4 candidates - full mesh of votes.
	// Because frame 3 candidates had half-mesh strongly reaching with the frame 2
	// this again delays the election of frame 1 candidates. Frame 2 should also
	// be undecided, waiting for the election of frame 1 to finish.
	addFrameCandidates(false)
	for i := range 2 {
		for _, candidate := range layers[i] {
			require.Equal(layering.VerdictUndecided, lachesis.IsLeader(dag, candidate))
		}
	}

	// Frame 5 candidates.
	addFrameCandidates(false)
	// Creator 0 should be elected leader as frame 5 candidates aggregate votes (full mesh)
	// from frame 4, which all voted positively for creator 0 (through a simple majority
	// aggregation of frame 3).
	require.Equal(layering.VerdictYes, lachesis.IsLeader(dag, layers[0][0]))
	// All other candidates should be ruled out.
	for _, candidate := range layers[0][1:] {
		require.Equal(layering.VerdictNo, lachesis.IsLeader(dag, candidate))
	}
	// Frame 2 candidates should be decided as well as there is a full mesh
	// of votes from frame 4 to frame 3 and frame 1 has been decided.
	require.Equal(layering.VerdictYes, lachesis.IsLeader(dag, layers[1][0]))
	for _, candidate := range layers[1][1:] {
		require.Equal(layering.VerdictNo, lachesis.IsLeader(dag, candidate))
	}

}

func TestLachesis_SortLeaders_ReturnsLeadersSortedByFrame(t *testing.T) {
	require := require.New(t)

	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{1: 1, 2: 1})
	require.NoError(err)

	lachesis := (&Factory{}).NewLayering(committee)
	dag := model.NewDag()

	events := []*model.Event{}
	expectedLeaders := []*model.Event{}

	lastEventCreator1 := createEventAndAddToDag(t, dag, 1, nil)
	lastEventCreator2 := createEventAndAddToDag(t, dag, 2, nil)
	events = append(events, lastEventCreator1, lastEventCreator2)

	for range 20 {
		lastEventCreator1 = createEventAndAddToDag(t, dag, 1, []*model.Event{lastEventCreator1, lastEventCreator2})
		lastEventCreator2 = createEventAndAddToDag(t, dag, 2, []*model.Event{lastEventCreator2, lastEventCreator1})
		events = append(events, lastEventCreator1, lastEventCreator2)
	}

	// Both creators have same stake, so the creator 1 will always win tie-breaks
	// due to lower CreatorId.
	for eventIterator := lastEventCreator1; eventIterator != nil; eventIterator = eventIterator.SelfParent() {
		if lachesis.IsLeader(dag, eventIterator) == layering.VerdictYes {
			expectedLeaders = append(expectedLeaders, eventIterator)
		}
	}
	slices.Reverse(expectedLeaders)

	rand.Shuffle(len(events), func(i, j int) {
		events[i], events[j] = events[j], events[i]
	})

	sortedLeaders := lachesis.SortLeaders(dag, events)
	require.Equal(expectedLeaders, sortedLeaders)
}

func createEventAndAddToDag(t *testing.T, dag *model.Dag, creator model.CreatorId, parents []*model.Event) *model.Event {
	t.Helper()

	parentIds := make([]model.EventId, 0, len(parents))
	for _, parent := range parents {
		parentIds = append(parentIds, parent.EventId())
	}
	newEvents := dag.AddEvent(model.EventMessage{Creator: creator, Parents: parentIds})
	require.Len(t, newEvents, 1)
	require.NotNil(t, newEvents[0])

	return newEvents[0]
}
