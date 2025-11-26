package moira

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var _ layering.Factory = Factory{}

func TestFactory_String_ProducesReadableSummary(t *testing.T) {
	factory := Factory{}
	require.Equal(t, "custom_moira", factory.String())
}

func TestMoira_IsALayeringImplementation(t *testing.T) {
	var _ layering.Layering = &Moira{}
}

func TestMoira_newMoira_DefaultsRelationsWhenOmitted(t *testing.T) {
	dag := model.NewMockDag(gomock.NewController(t))
	committee := consensus.NewUniformCommittee(2)

	// Cannot compare functions directly, so test by calling them on mocked dag.

	moira := newMoira(nil, dag, committee)
	dag.EXPECT().Reaches(gomock.Any(), gomock.Any()).Times(2)
	moira.CandidateLayerRelation(nil, nil)
	moira.VotingLayerRelation(nil, nil)

	moira = newMoira(&Factory{}, dag, committee)
	dag.EXPECT().Reaches(gomock.Any(), gomock.Any()).Times(2)
	moira.CandidateLayerRelation(nil, nil)
	moira.VotingLayerRelation(nil, nil)
}

func TestMoira_IsCandidate_ReturnsFalseForIllegalEvents(t *testing.T) {
	require := require.New(t)

	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{1: 1})
	require.NoError(err)

	moira := (&Factory{}).NewLayering(model.NewDag(committee), committee)
	require.False(moira.IsCandidate(nil))

	event, err := model.NewEvent(2, nil, nil)
	require.NoError(err)
	require.False(moira.IsCandidate(event))
}

func TestMoira_quorumOfRelations_OddTotalStake(t *testing.T) {
	committee := consensus.NewUniformCommittee(11)

	testMoira_quorumOfRelations(t, committee)
}

func TestMoira_quorumOfRelations_EvenTotalStake(t *testing.T) {
	committee := consensus.NewUniformCommittee(12)

	testMoira_quorumOfRelations(t, committee)
}

func testMoira_quorumOfRelations(t *testing.T, committee *consensus.Committee) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	dag := model.NewMockDag(ctrl)
	moira := newMoira(&Factory{}, dag, committee)

	source, err := model.NewEvent(1, nil, nil)
	require.NoError(err)

	targets := make([]*model.Event, 0, len(committee.Validators()))
	for i := 1; i <= len(committee.Validators()); i++ {
		e, err := model.NewEvent(consensus.ValidatorId(i), nil, nil)
		require.NoError(err)
		targets = append(targets, e)
	}

	for numStronglyReachedEvents := 0; numStronglyReachedEvents <= len(targets); numStronglyReachedEvents++ {
		t.Run(fmt.Sprint("number of strongly reached bases: ", numStronglyReachedEvents), func(t *testing.T) {
			// Use StronglyReaches relation for testing quorumOfRelations.
			for idx, target := range targets {
				dag.EXPECT().StronglyReaches(source, target).Return(idx < numStronglyReachedEvents)
			}

			expected := numStronglyReachedEvents >= len(targets)*2/3+1
			require.Equal(expected, moira.quorumOfRelations(source, targets, dag.StronglyReaches))
		})
	}
}

func TestMoira_IsLeader_RejectsHighestPriorityCandidate(t *testing.T) {
	require := require.New(t)

	const numCreators = 4
	committee := consensus.NewUniformCommittee(numCreators)

	dag := newMockedDag(t, committee)
	moira := newMoira(
		&Factory{CandidateLayerRelation: dag.StronglyReaches, VotingLayerRelation: dag.StronglyReaches},
		dag,
		committee,
	)

	// layers[frame-1][CreatorId]
	layers := make([][]*model.Event, 1)
	// Genesis events (Frame-1 Layer of candidates).
	for i := range numCreators {
		layers[0] = append(layers[0], createEventAndAddToDag(t, dag, consensus.ValidatorId(i), nil))
	}

	// Frame 2 candidates - quorum of Frame 2 candidates can't strongly reach Frame
	// 1 Creator 0 genesis candidate.
	// We want the creator 0 to be ruled out as a leader, so we make all frame 2
	// candidates (except from its own creator) vote negatively for it.
	layers = append(layers, newFrameCandidates(
		t, moira, dag, layers,
		func(candidateId, parentId consensus.ValidatorId) bool { return candidateId != 0 && parentId == 0 },
	))
	// Every Frame 1 candidate should be Undecided as no aggregating voters are present.
	for _, candidate := range layers[0] {
		require.Equal(layering.VerdictUndecided, moira.IsLeader(candidate))
	}

	// Frame 3 candidates - all candidates strongly reach all Frame 2 candidates.
	// filterOut is a no-op.
	layers = append(
		layers,
		newFrameCandidates(t, moira, dag, layers, func(_, _ consensus.ValidatorId) bool { return false }),
	)
	// The Creator 0 candidate should be ruled out as a leader and Creator 1
	// should be elected as it has the next highest priority.
	require.Equal(layering.VerdictNo, moira.IsLeader(layers[0][0]))
	require.Equal(layering.VerdictYes, moira.IsLeader(layers[0][1]))
}

func TestMoira_IsLeader_FrameElectionDelayedByLowerUndecidedFrame(t *testing.T) {
	require := require.New(t)

	const numCreators = 4
	committee := consensus.NewUniformCommittee(numCreators)

	dag := newMockedDag(t, committee)
	moira := newMoira(
		&Factory{CandidateLayerRelation: dag.StronglyReaches, VotingLayerRelation: dag.StronglyReaches},
		dag,
		committee,
	)

	// layers[frame-1][CreatorId]
	layers := make([][]*model.Event, 1)
	// Genesis events (Frame-1 Layer of candidates).
	for i := range numCreators {
		layers[0] = append(layers[0], createEventAndAddToDag(t, dag, consensus.ValidatorId(i), nil))
	}

	// We want the creator 0 candidate to receive only 50% of the votes from frame
	// above, so it can't be instantly elected as leader when candidates are aggregating
	// votes in a later frame.
	// This delays the election and and should result in all elections being Undecided.
	// To this end, we remove votes for creator-0 candidate from half of validator votes.
	halfMeshFilterOutFunc := func(candidateId, parentId consensus.ValidatorId) bool {
		return candidateId%2 == 1 && parentId == 0
	}
	// Frame 2 candidates - half of candidates strongly reach creator 0 genesis candidate.
	layers = append(layers, newFrameCandidates(t, moira, dag, layers, halfMeshFilterOutFunc))

	// Every Frame-1 candidate should be Undecided as no aggregating voters are present.
	for _, candidate := range layers[0] {
		require.Equal(layering.VerdictUndecided, moira.IsLeader(candidate))
	}

	// Frame 3 candidates - half of candidates strongly reach creator 0 frame 1 candidate.
	// This will delay the election of frame 1 candidates as well.
	layers = append(layers, newFrameCandidates(t, moira, dag, layers, halfMeshFilterOutFunc))
	// Every candidate should be Undecided as full quorum for Creator 0 (highest priority),
	// cannot be reached due to the missing votes from frame 2. The frame 3 aggregators,
	// while unable to make a decision on frame 0 candidates, still vote positively for
	// creator 0 due to presence simple majority (50 % of votes).
	for _, candidate := range layers[0] {
		require.Equal(layering.VerdictUndecided, moira.IsLeader(candidate))
	}

	// Frame 4 candidates - full mesh of votes.
	// Because frame 3 candidates had half-mesh strongly reaching with the frame 2
	// this again delays the election of frame 1 candidates. Frame 2 should also
	// be undecided, waiting for the election of frame 1 to finish.
	noOpFilterOutFunc := func(_, _ consensus.ValidatorId) bool { return false }
	layers = append(layers, newFrameCandidates(t, moira, dag, layers, noOpFilterOutFunc))
	for i := range 2 {
		for _, candidate := range layers[i] {
			require.Equal(layering.VerdictUndecided, moira.IsLeader(candidate))
		}
	}

	// Frame 5 candidates.
	layers = append(layers, newFrameCandidates(t, moira, dag, layers, noOpFilterOutFunc))
	// Creator 0 should be elected leader as frame 5 candidates aggregate votes (full mesh)
	// from frame 4, which all voted positively for creator 0 (through a simple majority
	// aggregation of frame 3).
	require.Equal(layering.VerdictYes, moira.IsLeader(layers[0][0]))
	// All other candidates should be ruled out.
	for _, candidate := range layers[0][1:] {
		require.Equal(layering.VerdictNo, moira.IsLeader(candidate))
	}
	// Frame 2 candidates should be decided as well as there is a full mesh
	// of votes from frame 4 to frame 3 and frame 1 has been decided.
	require.Equal(layering.VerdictYes, moira.IsLeader(layers[1][0]))
	for _, candidate := range layers[1][1:] {
		require.Equal(layering.VerdictNo, moira.IsLeader(candidate))
	}
}

func TestMoira_IsLeader_FrameElectionDelayedByLackOfQuorum(t *testing.T) {
	require := require.New(t)

	const numCreators = 5
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{0: 1, 1: 1, 2: 1, 3: 1, 4: 1})
	require.NoError(err)

	dag := newMockedDag(t, committee)
	moira := newMoira(
		&Factory{CandidateLayerRelation: dag.StronglyReaches, VotingLayerRelation: dag.StronglyReaches},
		dag,
		committee,
	)

	// layers[frame-1][CreatorId]
	layers := make([][]*model.Event, 1)
	// Genesis events (Frame-1 Layer of candidates).
	for i := range numCreators {
		layers[0] = append(layers[0], createEventAndAddToDag(t, dag, consensus.ValidatorId(i), nil))
	}

	// Half mesh would result in 3 yes and 2 no votes for creator 0 candidate
	halfMeshFilterOutFunc := func(candidateId, parentId consensus.ValidatorId) bool {
		return candidateId%2 == 1 && parentId == 0
	}
	// All of the candidates strongly reach creator 0 frame 1 candidate.
	lastValidatorFilterOutFunc := func(candidateId, parentId consensus.ValidatorId) bool {
		lastValidatorId := consensus.ValidatorId(numCreators - 1)
		return candidateId != lastValidatorId && parentId == lastValidatorId
	}
	noOpFilterOutFunc := func(_, _ consensus.ValidatorId) bool { return false }

	// Frame 2 candidates - 3/5 of candidates strongly reach creator 0 genesis candidate.
	layers = append(layers, newFrameCandidates(t, moira, dag, layers, halfMeshFilterOutFunc))
	// Every Frame-1 candidate should be Undecided as no aggregating voters are present.
	for _, candidate := range layers[0] {
		require.Equal(layering.VerdictUndecided, moira.IsLeader(candidate))
	}

	// Frame 3 candidates - 3/5 of candidates strongly reach creator 0 frame 1 candidate.
	// This will delay the election of frame 1 candidates, but the aggregators
	// will be voting positively for creator 0 due to presence simple majority.
	layers = append(layers, newFrameCandidates(t, moira, dag, layers, lastValidatorFilterOutFunc))
	// Every candidate should be Undecided as full quorum for Creator 0 (highest priority),
	// cannot be reached due to the missing votes from frame 2.
	for _, candidate := range layers[0] {
		require.Equal(layering.VerdictUndecided, moira.IsLeader(candidate))
	}

	// Frame 4 candidates - full mesh of votes.
	// Simple majority votes from frame 3 should be aggregated in frame 4,
	// electing creator 0 candidate as leader.
	layers = append(layers, newFrameCandidates(t, moira, dag, layers, noOpFilterOutFunc))
	require.Equal(layering.VerdictYes, moira.IsLeader(layers[0][0]))
	for _, candidate := range layers[0][1:] {
		require.Equal(layering.VerdictNo, moira.IsLeader(candidate))
	}
}

// newFrameCandidates creates a new layer of candidate events, one per creator.
// The candidates are created as children of the previous layer's events,
// with optional filtering of parents to simulate missing strongly reaches
// relationships.
// It is assumed that the previous layer contains exactly one event per creator,
// and that there is always a layer before the new one.
func newFrameCandidates(
	t *testing.T,
	layering layering.Layering,
	dag *model.MockDag,
	layers [][]*model.Event,
	filterOut func(creatorId, parentId consensus.ValidatorId) bool,
) []*model.Event {
	t.Helper()
	frameIdx := len(layers)
	newLayer := []*model.Event{}
	for creatorId := range consensus.ValidatorId(len(layers[0])) {
		prevLayer := slices.Clone(layers[frameIdx-1])
		// Move own creator event to the front.
		prevLayer[0], prevLayer[creatorId] = prevLayer[creatorId], prevLayer[0]
		parents := slices.DeleteFunc(slices.Clone(prevLayer), func(parent *model.Event) bool { return filterOut(creatorId, parent.Creator()) })
		event := createEventAndAddToDag(t, dag, consensus.ValidatorId(creatorId), parents)
		// Simulating candidate status by priming the stronglyReachesCache.
		for _, base := range prevLayer {
			dag.EXPECT().StronglyReaches(event, base).Return(slices.Contains(parents, base)).AnyTimes()
		}
		require.True(t, layering.IsCandidate(event))
		newLayer = append(newLayer, event)
	}
	return newLayer
}

func createEventAndAddToDag(t *testing.T, dag model.Dag, creator consensus.ValidatorId, parents []*model.Event) *model.Event {
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

// newMockedDag creates a mocked Dag instance that delegates irrelevant methods
// to an underlying real Dag instance for simplicity.
func newMockedDag(t *testing.T, committee *consensus.Committee) *model.MockDag {
	t.Helper()

	ctrl := gomock.NewController(t)
	underlyingDag := model.NewDag(committee)
	dag := model.NewMockDag(ctrl)
	dag.EXPECT().AddEvent(gomock.Any()).DoAndReturn(underlyingDag.AddEvent).AnyTimes()
	dag.EXPECT().GetHeads().DoAndReturn(underlyingDag.GetHeads).AnyTimes()

	return dag
}

func TestMoira_SortLeaders_ReturnsLeadersSortedByFrame(t *testing.T) {
	require := require.New(t)

	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{1: 1, 2: 1})
	require.NoError(err)

	dag := model.NewDag(committee)
	moira := (&Factory{
		CandidateLayerRelation: dag.StronglyReaches,
		VotingLayerRelation:    dag.StronglyReaches,
	}).NewLayering(dag, committee)

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
		if moira.IsLeader(eventIterator) == layering.VerdictYes {
			expectedLeaders = append(expectedLeaders, eventIterator)
		}
	}
	slices.Reverse(expectedLeaders)

	rand.Shuffle(len(events), func(i, j int) {
		events[i], events[j] = events[j], events[i]
	})

	sortedLeaders := moira.SortLeaders(events)
	require.Equal(expectedLeaders, sortedLeaders)
}

func TestMoira_IsCandidate_TrueForFirstInFrameCandidate(t *testing.T) {
	require := require.New(t)

	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{1: 1, 2: 1})
	require.NoError(err)

	dag := model.NewDag(committee)
	moira := (&Factory{
		CandidateLayerRelation: dag.StronglyReaches,
		VotingLayerRelation:    dag.StronglyReaches,
	}).NewLayering(dag, committee)
	// e_#creatorid_#seq
	// c - candidate

	// e_1_3(c)═══╬
	// ║          ║
	// ╬════════e_2_2(c)
	// ║          ║
	// e_1_2══════╬
	// ║          ║
	// e_1_1(c) e_2_1(c)

	e_1_1 := createEventAndAddToDag(t, dag, 1, nil)
	e_2_1 := createEventAndAddToDag(t, dag, 2, nil)
	e_1_2 := createEventAndAddToDag(t, dag, 1, []*model.Event{e_1_1, e_2_1})

	require.True(moira.IsCandidate(e_1_1))
	require.True(moira.IsCandidate(e_2_1))
	require.False(moira.IsCandidate(e_1_2))

	e_2_2 := createEventAndAddToDag(t, dag, 2, []*model.Event{e_2_1, e_1_2})
	e_1_3 := createEventAndAddToDag(t, dag, 1, []*model.Event{e_1_2, e_2_2})

	require.True(moira.IsCandidate(e_2_2))
	require.True(moira.IsCandidate(e_1_3))
}

func TestMoira_IsLeader_ElectsLeadersSequentiallyByFrames(t *testing.T) {
	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{1: 2, 2: 1})
	require.NoError(t, err)

	dag := model.NewDag(committee)
	moira := (&Factory{
		CandidateLayerRelation: dag.StronglyReaches,
		VotingLayerRelation:    dag.StronglyReaches,
	}).NewLayering(dag, committee)

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
		require.Equal(t, layering.VerdictUndecided, moira.IsLeader(e_1_1))
		require.Equal(t, layering.VerdictUndecided, moira.IsLeader(e_2_1))
		require.Equal(t, layering.VerdictUndecided, moira.IsLeader(e_2_2))
		require.Equal(t, layering.VerdictUndecided, moira.IsLeader(e_1_3))
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
		require.Equal(t, layering.VerdictUndecided, moira.IsLeader(e_2_3))
		// e_1_4 doesn't strongly reach e_1_3, and can't gather a quorum to become a candidate.
		require.Equal(t, layering.VerdictNo, moira.IsLeader(e_1_4))
		// e1_1_1 is elected leader by e_2_3 aggregating votes from e_1_3 and e_2_2
		require.Equal(t, layering.VerdictYes, moira.IsLeader(e_1_1))
		// and e_2_1 is ruled out.
		require.Equal(t, layering.VerdictNo, moira.IsLeader(e_2_1))
	})
}
