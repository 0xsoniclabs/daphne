package tendermint

import (
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTendermint_Factory_ImplementsConsensusFactory(t *testing.T) {
	var _ consensus.Factory = Factory{}
}

func TestTendermint_Factory_StringSummarizesConfiguration(t *testing.T) {
	require := require.New(t)
	factory := &Factory{}
	require.Equal(
		"tendermint_proposeTO=500ms_prevoteTO=500ms_precommitTO=500ms_phaseDelta=0s",
		factory.String(),
	)
}

func TestTendermint_NewActive_InstantiatesActiveTendermintAndRegistersListenersAndStartsFinalizing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		leaderId := p2p.PeerId("leader")
		network := p2p.NewNetwork()
		server, err := network.NewServer(leaderId)
		require.NoError(t, err)

		leaderCreatorId := consensus.ValidatorId(1)
		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			leaderCreatorId: 1,
		})
		require.NoError(t, err)

		transactions := []types.Transaction{}
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)
		mockReceiver := consensus.NewMockBundleListener(ctrl)
		mockReceiver.EXPECT().OnNewBundle(gomock.Any()).Times(1)

		factory := &Factory{
			HeightLimit: 1,
		}
		tm := factory.NewActive(server, *committee, leaderCreatorId, mockSource).(*Tendermint)
		tm.RegisterListener(mockReceiver)

		synctest.Wait()
	})
}

func TestTendermint_SinglePassiveNodeFinalizesBlocksWhenReceivingThemFromActiveNode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		network := p2p.NewNetwork()
		activeServer, err := network.NewServer(p2p.PeerId("active"))
		require.NoError(t, err)
		passiveServer, err := network.NewServer(p2p.PeerId("passive"))
		require.NoError(t, err)

		activeId := consensus.ValidatorId(1)
		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			activeId: 1,
		})
		require.NoError(t, err)

		transactions := []types.Transaction{}
		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)
		mockReceiver := consensus.NewMockBundleListener(ctrl)
		mockReceiver.EXPECT().OnNewBundle(gomock.Any()).Times(1)

		// Create active Tendermint node
		factory := &Factory{
			HeightLimit: 1,
		}
		// Create passive Tendermint node
		passiveTm := factory.NewPassive(passiveServer, *committee).(*Tendermint)
		passiveTm.RegisterListener(mockReceiver)
		_ = factory.NewActive(activeServer, *committee, activeId, mockSource).(*Tendermint)

		// Wait some time for the nodes to stop.
		time.Sleep(time.Second * 1)
	})
}

func TestTendermint_VotesOnProposalThatHadPolkaInPreviousRound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		network := p2p.NewNetwork()
		server, err := network.NewServer(p2p.PeerId("server"))
		require.NoError(t, err)

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
			2: 1,
		})
		require.NoError(t, err)

		transactions := []types.Transaction{}
		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateTransactions().Return(transactions).MinTimes(1)

		factory := &Factory{}

		tm := factory.NewActive(server, *committee, 1, source).(*Tendermint)
		// 1 proposes a block in round 0.
		// Receives its own proposal, triggering freshProposalRule.
		synctest.Wait()
		proposal := tm.getProposalBlock()
		// Get a quorum of prevotes, observing a polka for it and triggering
		// observePolkaOnProposalRule.
		tm.fakeHandleMessage(Message{
			Phase:     Prevote,
			Height:    0,
			Round:     0,
			BlockId:   proposal.Id(),
			Signature: 2,
		})
		// A nil precommit gets sent - no precommit quorum on proposal.
		tm.fakeHandleMessage(Message{
			Phase:     Precommit,
			Height:    0,
			Round:     0,
			BlockId:   types.Hash{},
			Signature: 2,
		})
		// Timeout gets triggered, so waiting for it to finish.
		time.Sleep(DefaultPhaseTimeout)
		synctest.Wait()
		// Now, the node should be in round 1. It should vote for the proposal
		// that had the polka in round 0, thus triggering the polkaProposalRule.
		// 2 will repeat 1's proposal in round 1, also voting for it.
		tm.fakeHandleMessage(Message{
			Phase:      Propose,
			Height:     0,
			Round:      1,
			Block:      proposal,
			PolkaRound: 0,
			Signature:  2,
		})
		tm.fakeHandleMessage(Message{
			Phase:     Prevote,
			Height:    0,
			Round:     1,
			BlockId:   proposal.Id(),
			Signature: 2,
		})
		synctest.Wait()
		// Check that it voted for the proposal.
		tm.stateMutex.Lock()
		require.True(t, tm.predicateHasQuorum(func(msg Message) bool {
			return msg.Phase == Prevote &&
				msg.Height == 0 &&
				msg.Round == 1 &&
				msg.BlockId == proposal.Id()
		}, 0))
		tm.stateMutex.Unlock()
		tm.Stop()
	})
}

func TestTendermint_PrevotesNilIfNoProposalHasBeenReceivedInTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		network := p2p.NewNetwork()
		server, err := network.NewServer(p2p.PeerId("server"))
		require.NoError(t, err)

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
			2: 1,
		})
		require.NoError(t, err)

		source := consensus.NewMockTransactionProvider(ctrl)

		factory := &Factory{}
		tm := factory.NewActive(server, *committee, 2, source).(*Tendermint)
		// No proposal is received in round 0. Timeout gets triggered,
		// so waiting for it to finish.
		time.Sleep(DefaultPhaseTimeout)
		// A nil prevote is received by 1.
		tm.fakeHandleMessage(Message{
			Phase:     Prevote,
			Height:    0,
			Round:     0,
			BlockId:   types.Hash{},
			Signature: 1,
		})
		synctest.Wait()
		// Check that it voted nil.
		tm.stateMutex.Lock()
		require.True(t, tm.predicateHasQuorum(func(msg Message) bool {
			return msg.Phase == Prevote &&
				msg.Height == 0 &&
				msg.Round == 0 &&
				msg.BlockId == types.Hash{}
		}, 0))
		tm.stateMutex.Unlock()
		tm.Stop()
	})
}

func TestTendermint_PrecommitsNilIfNoPolkaIsObservedInPrevotePhase(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		network := p2p.NewNetwork()
		server, err := network.NewServer(p2p.PeerId("server"))
		require.NoError(t, err)

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
			2: 1,
		})
		require.NoError(t, err)

		source := consensus.NewMockTransactionProvider(ctrl)

		factory := &Factory{}
		tm := factory.NewActive(server, *committee, 2, source).(*Tendermint)
		synctest.Wait()
		// A proposal is received in round 0 (but not the vote!).
		tm.fakeHandleMessage(Message{
			Phase:  Propose,
			Height: 0,
			Round:  0,
			Block: &Block{
				Transactions: []types.Transaction{},
				Number:       1,
			},
			PolkaRound: -1,
			Signature:  1,
		})
		// A nil prevote is received instead.
		tm.fakeHandleMessage(Message{
			Phase:     Prevote,
			Height:    0,
			Round:     0,
			BlockId:   types.Hash{},
			Signature: 1,
		})
		// Timeout gets triggered, so waiting for it to finish.
		time.Sleep(DefaultPhaseTimeout)
		synctest.Wait()
		// Check that it precommitted nil.
		tm.stateMutex.Lock()
		require.True(t, tm.predicateHasAtLeastOneHonestVote(func(msg Message) bool {
			return msg.Phase == Precommit &&
				msg.Round == 0 &&
				msg.BlockId == types.Hash{} &&
				msg.Signature == 2
		}, 0))
		tm.stateMutex.Unlock()
		tm.Stop()
	})
}

func TestTendermint_CatchesUpAfterReceivingMessagesFromFutureRounds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		network := p2p.NewNetwork()
		server, err := network.NewServer(p2p.PeerId("server"))
		require.NoError(t, err)

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
			2: 1,
		})
		require.NoError(t, err)

		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateTransactions().Return([]types.Transaction{}).AnyTimes()

		factory := &Factory{}
		tm := factory.NewActive(server, *committee, 2, source).(*Tendermint)
		synctest.Wait()
		// Receives messages from round 2 before rounds 0 and 1 have completed.
		tm.fakeHandleMessage(Message{
			Phase:  Propose,
			Height: 0,
			Round:  2,
			Block: &Block{
				Transactions: []types.Transaction{},
				Number:       1,
			},
			PolkaRound: -1,
			Signature:  1,
		})
		synctest.Wait()
		tm.stateMutex.Lock()
		require.Equal(t, 2, tm.round)
		tm.stateMutex.Unlock()
		tm.Stop()
	})
}

func TestTendermint_StopIsIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		network := p2p.NewNetwork()
		server, err := network.NewServer(p2p.PeerId("server"))
		require.NoError(t, err)

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
		})
		require.NoError(t, err)

		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateTransactions().Return([]types.Transaction{}).AnyTimes()
		factory := &Factory{}
		tm := factory.NewActive(server, *committee, 1, source).(*Tendermint)

		// Call Stop multiple times and ensure no panic or error.
		require.NotPanics(t, func() {
			tm.Stop()
			tm.Stop()
			tm.Stop()
		})
	})
}

func TestMessage_GetMessageType_ReturnsTendermintMessageType(t *testing.T) {
	var msg Message
	require.Equal(t, p2p.MessageType("Tendermint"), msg.GetMessageType())
}

func TestMessage_MessageSize_ReturnsCorrectSize(t *testing.T) {
	tx1 := types.Transaction{}
	block := &Block{
		Transactions: []types.Transaction{tx1},
		Number:       1,
	}
	msg1 := Message{
		Phase:     Prevote,
		Height:    0,
		Round:     0,
		BlockId:   block.Id(),
		Block:     nil,
		Signature: 1,
	}
	msg2 := Message{
		Phase:     Prevote,
		Height:    0,
		Round:     0,
		BlockId:   block.Id(),
		Block:     block,
		Signature: 1,
	}

	sizeWithoutBlock := msg1.MessageSize()
	sizeWithBlock := msg2.MessageSize()

	require.Equal(t, sizeWithoutBlock+
		uint32(reflect.TypeFor[Block]().Size())+tx1.MessageSize(), sizeWithBlock,
	)
}

// An auxiliary method to fake-handle messages with proper locking in tests.
func (t *Tendermint) fakeHandleMessage(msg Message) {
	t.stateMutex.Lock()
	defer t.stateMutex.Unlock()
	t.handleMessage(msg)
}

func (t *Tendermint) getProposalBlock() *Block {
	t.stateMutex.Lock()
	defer t.stateMutex.Unlock()
	if t.getCurrentProposalMessage() != nil {
		return t.getCurrentProposalMessage().Block
	}
	return nil
}
