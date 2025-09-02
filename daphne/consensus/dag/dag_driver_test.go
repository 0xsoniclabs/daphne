package dag

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDagConsensus_NewActive_ActiveInstanceEmitsEvents(t *testing.T) {
	ctrl := gomock.NewController(t)

	const numEmissions = 5

	layeringProtocol := layering.NewMockLayering(ctrl)
	layeringProtocol.EXPECT().IsCandidate(gomock.Any()).Return(false).AnyTimes()
	layeringProtocol.EXPECT().SortLeaders(gomock.Any(), gomock.Len(0)).AnyTimes()

	transactionSource := consensus.NewMockTransactionProvider(ctrl)
	transactionSource.EXPECT().GetCandidateTransactions().Return([]types.Transaction{{}}).Times(numEmissions)

	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())
	server.EXPECT().GetPeers().Times(numEmissions)

	synctest.Test(t, func(t *testing.T) {
		c := newActiveDagConsensus(server, layeringProtocol, 1, transactionSource, generic.DefaultEmitInterval)
		time.Sleep(numEmissions * generic.DefaultEmitInterval)
		c.Stop()
	})
}

func TestDagConsensus_processEventMessage_IgnoresAlreadySeenEvent(t *testing.T) {
	ctrl := gomock.NewController(t)

	layeringProtocol := layering.NewMockLayering(ctrl)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())
	server.EXPECT().GetPeers().AnyTimes()

	consensus, _ := newPassiveDagConsensus(server, layeringProtocol)

	event := model.EventMessage{Creator: 1}
	// Only a single call to IsCandidate is made.
	layeringProtocol.EXPECT().IsCandidate(model.WithEventId(event.EventId())).Return(false)
	layeringProtocol.EXPECT().SortLeaders(gomock.Any(), gomock.Len(0))
	// No calls to IsLeader are made.

	consensus.processEventMessage(event)
	consensus.processEventMessage(event)
}

func TestDagConsensus_processEventMessage_DiscardsNonCandidateEvents(t *testing.T) {
	ctrl := gomock.NewController(t)

	layeringProtocol := layering.NewMockLayering(ctrl)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())
	server.EXPECT().GetPeers().AnyTimes()

	consensus, _ := newPassiveDagConsensus(server, layeringProtocol)

	event := model.EventMessage{Creator: 1}
	// The event is not a candidate.
	layeringProtocol.EXPECT().IsCandidate(model.WithEventId(event.EventId())).Return(false)
	layeringProtocol.EXPECT().SortLeaders(gomock.Any(), gomock.Len(0))
	// No IsLeader calls are made.

	consensus.processEventMessage(event)
	// The event is not stored.
	require.Empty(t, consensus.leaderCandidates)
}

func TestDagConsensus_processEventMessage_MaintainsPotentialLeaders(t *testing.T) {
	ctrl := gomock.NewController(t)

	layeringProtocol := layering.NewMockLayering(ctrl)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())
	server.EXPECT().GetPeers().AnyTimes()

	consensus, _ := newPassiveDagConsensus(server, layeringProtocol)

	event := model.EventMessage{}
	layeringProtocol.EXPECT().IsCandidate(model.WithEventId(event.EventId())).Return(true)
	// A call to IsLeader is made, and the event's leader status is reported as undecided.
	layeringProtocol.EXPECT().IsLeader(gomock.Any(), model.WithEventId(event.EventId())).Return(layering.VerdictUndecided)
	layeringProtocol.EXPECT().SortLeaders(gomock.Any(), gomock.Len(0))

	consensus.processEventMessage(event)
	// The event gets stored as a potential leader.
	require.Len(t, consensus.leaderCandidates, 1)
	require.Equal(t, event.EventId(), consensus.leaderCandidates[0].EventId())
}

func TestDagConsensus_processEventMessage_DeliversBundlesWhileMaintainingConsistentState(t *testing.T) {
	ctrl := gomock.NewController(t)

	layeringProtocol := layering.NewMockLayering(ctrl)
	server := p2p.NewMockServer(ctrl)
	server.EXPECT().RegisterMessageHandler(gomock.Any())
	server.EXPECT().GetPeers().AnyTimes()
	listener := consensus.NewMockBundleListener(ctrl)

	consensus, _ := newPassiveDagConsensus(server, layeringProtocol)

	consensus.RegisterListener(listener)

	event1 := model.EventMessage{Creator: 1}
	event2 := model.EventMessage{Creator: 2}

	// Event 1 is initially a potential leader.
	layeringProtocol.EXPECT().IsCandidate(model.WithEventId(event1.EventId())).Return(true)
	layeringProtocol.EXPECT().IsLeader(gomock.Any(), model.WithEventId(event1.EventId())).Return(layering.VerdictUndecided)
	layeringProtocol.EXPECT().SortLeaders(gomock.Any(), gomock.Len(0))
	consensus.processEventMessage(event1)

	// Event 2 is instantly a leader and "promotes" event 1 to a leader as well.
	layeringProtocol.EXPECT().IsCandidate(model.WithEventId(event2.EventId())).Return(true)
	layeringProtocol.EXPECT().IsLeader(gomock.Any(), model.WithEventId(event1.EventId())).Return(layering.VerdictYes)
	layeringProtocol.EXPECT().IsLeader(gomock.Any(), model.WithEventId(event2.EventId())).Return(layering.VerdictYes)
	// SortLeaders should be called on both events.
	layeringProtocol.EXPECT().SortLeaders(gomock.Any(), gomock.Len(2)).Return([]*model.Event{{}, {}})

	// Both events should trigger the bundle listener.
	listener.EXPECT().OnNewBundle(gomock.Any()).Times(2)
	consensus.processEventMessage(event2)

	// Candidate evidence should end up empty.
	require.Empty(t, consensus.leaderCandidates)
	// The next bundle number should be incremented once for each event.
	require.Equal(t, uint32(2), consensus.nextBundleNumber)
}
