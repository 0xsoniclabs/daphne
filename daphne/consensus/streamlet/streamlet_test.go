package streamlet

import (
	"testing"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStreamlet_NewActive_InstatiatesActiveStreamletAndRegistersListenersAndStartsEmittingBundles(t *testing.T) {
	ctrl := gomock.NewController(t)

	leaderId := p2p.PeerId("leader")
	network := p2p.NewNetwork()
	server, err := network.NewServer(leaderId)
	require.NoError(t, err)

	leaderCreatorId := model.CreatorId(1)
	committee, err := consensus.NewCommittee(map[model.CreatorId]uint32{
		leaderCreatorId: 1,
	})
	require.NoError(t, err)

	config := Factory{
		EpochDuration: 1 * time.Second,
		Committee:     *committee,
		SelfId:        leaderCreatorId,
	}

	transactions := []types.Transaction{{From: 1, To: 2, Value: 10}}
	mockSource := consensus.NewMockTransactionProvider(ctrl)
	mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)

	mockListener := consensus.NewMockBundleListener(ctrl)
	mockListener.EXPECT().OnNewBundle(gomock.Any()).MinTimes(1)

	centralConsensus := config.NewActive(server, mockSource)
	centralConsensus.RegisterListener(mockListener)

	time.Sleep(2 * config.EpochDuration)
}
