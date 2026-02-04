package hotstuff

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/p2p/broadcast"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHotstuff_Factory_ImplementsConsensusFactory(t *testing.T) {
	var _ consensus.Factory = &Factory{}
}

func TestHotstuff_Factory_String_ReturnsSummary(t *testing.T) {
	config := Factory{Tau: time.Second, Delta: 100 * time.Millisecond}
	require.Equal(t, "hotstuff-tau=1000ms-delta=100ms", config.String())

	config = Factory{Tau: 500 * time.Millisecond, Delta: 50 * time.Millisecond}
	require.Equal(t, "hotstuff-tau=500ms-delta=50ms", config.String())
}

func TestHotstuff_Factory_NormalizeTimings_SetsDefaults(t *testing.T) {
	config := Factory{}
	config.normalizeTimings()
	require.Equal(t, defaultDelta, config.Delta)
	require.Equal(t, defaultTau, config.Tau)

	config = Factory{Delta: 200 * time.Millisecond}
	config.normalizeTimings()
	require.Equal(t, 200*time.Millisecond, config.Delta)
	require.Equal(t, 1400*time.Millisecond, config.Tau) // 7 * Delta

	config = Factory{Tau: 4 * time.Second}
	config.normalizeTimings()
	require.Equal(t, defaultDelta, config.Delta)
	require.Equal(t, 4*time.Second, config.Tau)
}

func TestHotstuff_Factory_NormalizeTimings_EnforcesTauMinimum(t *testing.T) {
	config := Factory{Tau: 100 * time.Millisecond, Delta: 200 * time.Millisecond}
	config.normalizeTimings()
	require.Equal(t, 200*time.Millisecond, config.Delta)
	require.Equal(t, 1400*time.Millisecond, config.Tau)

	config = Factory{Tau: 1400 * time.Millisecond, Delta: 200 * time.Millisecond}
	config.normalizeTimings()
	require.Equal(t, 200*time.Millisecond, config.Delta)
	require.Equal(t, 1400*time.Millisecond, config.Tau)

	config = Factory{Tau: 2 * time.Second, Delta: 200 * time.Millisecond}
	config.normalizeTimings()
	require.Equal(t, 200*time.Millisecond, config.Delta)
	require.Equal(t, 2*time.Second, config.Tau)
}

func TestHotstuff_NewActive_InstantiatesActiveHotstuffAndRegistersListenersAndStartsFinalizing(t *testing.T) {
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
		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(transactions).MinTimes(1)

		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)
		mockReceiver := consensus.NewMockBundleListener(ctrl)
		mockReceiver.EXPECT().OnNewBundle(gomock.Any()).Times(1)

		factory := &Factory{
			ViewLimit: 2,
		}
		hs := factory.NewActive(server, *committee, leaderCreatorId, mockSource).(*Hotstuff)
		hs.RegisterListener(mockReceiver)

		synctest.Wait()
	})
}

func TestHotstuff_SinglePassiveNodeReceivesBlocksFromActiveNode(t *testing.T) {
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
		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return(transactions).MinTimes(1)

		mockSource := consensus.NewMockTransactionProvider(ctrl)
		mockSource.EXPECT().GetCandidateLineup().Return(lineup).MinTimes(1)
		mockReceiver := consensus.NewMockBundleListener(ctrl)
		mockReceiver.EXPECT().OnNewBundle(gomock.Any()).Times(1)

		factory := &Factory{
			ViewLimit: 2,
		}
		passiveHs := factory.NewPassive(passiveServer, *committee).(*Hotstuff)
		passiveHs.RegisterListener(mockReceiver)
		_ = factory.NewActive(activeServer, *committee, activeId, mockSource).(*Hotstuff)

		synctest.Wait()
	})
}

func TestHotstuff_NonLeaderRespondsCorrectlyToMessagesByLeader(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		server.EXPECT().GetLocalId().AnyTimes()
		server.EXPECT().GetPeers().AnyTimes()

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
			2: 1,
		})
		require.NoError(t, err)

		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()
		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateLineup().Return(lineup).AnyTimes()

		newBlock := Block{
			PrevHash: genesisBlock(committee).Hash(),
			View:     1,
			Justify: certificate{
				view:       0,
				blockHash:  genesisBlock(committee).Hash(),
				signatures: getFullVoteCounter(committee),
			},
			Payload: []types.Transaction{},
		}

		gossip := broadcast.NewMockChannel[Message](ctrl)
		gossip.EXPECT().Register(gomock.Any()).AnyTimes()
		gossip.EXPECT().Unregister(gomock.Any()).AnyTimes()
		gomock.InOrder(
			// Sent on startup.
			gossip.EXPECT().Broadcast(Message{
				Signature: 1,
				Type:      NewView,
				View:      1,
				Contents: MessageNewViewContents{
					HighQC: certificate{
						view:       0,
						blockHash:  genesisBlock(committee).Hash(),
						signatures: getFullVoteCounter(committee),
					},
				},
			}),
			// Sent in response to Propose.
			gossip.EXPECT().Broadcast(Message{
				Signature: 1,
				Type:      Vote,
				View:      1,
				Contents: MessageVoteContents{
					BlockHash: newBlock.Hash(),
				},
			}),
		)
		factory := &Factory{
			getGossipFunction: func(p2p.Server) broadcast.Channel[Message] {
				return gossip
			},
			ViewLimit: 1,
		}
		hs := factory.NewActive(server, *committee, 1, source).(*Hotstuff)
		synctest.Wait()
		// Calls handleProposeRule.
		hs.fakeHandleMessage(Message{
			Signature: 2,
			Type:      Propose,
			View:      1,
			Contents: MessageProposeContents{
				Block: newBlock,
				High_QC: certificate{
					view:       0,
					blockHash:  genesisBlock(committee).Hash(),
					signatures: getFullVoteCounter(committee),
				},
				Commit_QC: certificate{},
			},
		})
		synctest.Wait()
		// Calls handlePrepareRule.
		hs.fakeHandleMessage(Message{
			Signature: 2,
			Type:      Prepare,
			View:      1,
			Contents: MessagePrepareContents{
				PrepareQC: certificate{
					view:       1,
					blockHash:  newBlock.Hash(),
					signatures: getFullVoteCounter(committee),
				},
			},
		})
		synctest.Wait()
		hs.Stop()
	})
}

func TestHotstuff_LeaderRespondsCorrectlyToMessagesByParticipants(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		server.EXPECT().GetLocalId().AnyTimes()
		server.EXPECT().GetPeers().AnyTimes()

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
			2: 1,
		})
		require.NoError(t, err)

		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()
		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateLineup().Return(lineup).AnyTimes()

		newBlock := Block{
			PrevHash: genesisBlock(committee).Hash(),
			View:     1,
			Justify: certificate{
				view:       0,
				blockHash:  genesisBlock(committee).Hash(),
				signatures: getFullVoteCounter(committee),
			},
			Payload: []types.Transaction{},
		}

		gossip := broadcast.NewMockChannel[Message](ctrl)
		gossip.EXPECT().Register(gomock.Any()).AnyTimes()
		gossip.EXPECT().Unregister(gomock.Any()).AnyTimes()
		gomock.InOrder(
			// Sent on startup.
			gossip.EXPECT().Broadcast(Message{
				Signature: 2,
				Type:      NewView,
				View:      1,
				Contents: MessageNewViewContents{
					HighQC: certificate{
						view:       0,
						blockHash:  genesisBlock(committee).Hash(),
						signatures: getFullVoteCounter(committee),
					},
				},
			}),
			// Sent in response to NewView quorum.
			gossip.EXPECT().Broadcast(Message{
				Signature: 2,
				Type:      Propose,
				View:      1,
				Contents: MessageProposeContents{
					Block: newBlock,
					High_QC: certificate{
						view:       0,
						blockHash:  genesisBlock(committee).Hash(),
						signatures: getFullVoteCounter(committee),
					},
					Commit_QC: certificate{
						signatures: getFullVoteCounter(committee),
					},
				},
			}),
			// Sent in response to Propose.
			gossip.EXPECT().Broadcast(Message{
				Signature: 2,
				Type:      Vote,
				View:      1,
				Contents: MessageVoteContents{
					BlockHash: newBlock.Hash(),
				},
			}),
			// Sent in response to Vote quorum.
			gossip.EXPECT().Broadcast(Message{
				Signature: 2,
				Type:      Prepare,
				View:      1,
				Contents: MessagePrepareContents{
					PrepareQC: certificate{
						view:       1,
						blockHash:  newBlock.Hash(),
						signatures: getFullVoteCounter(committee),
					},
				},
			}))
		validators := committee.Validators()

		factory := &Factory{
			getGossipFunction: func(p2p.Server) broadcast.Channel[Message] {
				return gossip
			},
			ViewLimit: 1,
		}
		hs := factory.NewActive(server, *committee, 2, source).(*Hotstuff)
		synctest.Wait()
		fakeHandleMessageBulk := func(msg Message, validators []consensus.ValidatorId) {
			for _, v := range validators {
				msg.Signature = v
				hs.fakeHandleMessage(msg)
			}
		}
		fakeHandleMessageBulk(Message{
			Type: NewView,
			View: 1,
			Contents: MessageNewViewContents{
				HighQC: certificate{
					view:       0,
					blockHash:  genesisBlock(committee).Hash(),
					signatures: getFullVoteCounter(committee),
				},
			},
		}, validators)
		synctest.Wait()
		hs.fakeHandleMessage(Message{
			Signature: 2,
			Type:      Propose,
			View:      1,
			Contents: MessageProposeContents{
				Block: newBlock,
				High_QC: certificate{
					view:       0,
					blockHash:  genesisBlock(committee).Hash(),
					signatures: getFullVoteCounter(committee),
				},
				Commit_QC: certificate{
					signatures: getFullVoteCounter(committee),
				},
			},
		})
		synctest.Wait()
		fakeHandleMessageBulk(Message{
			Type: Vote,
			View: 1,
			Contents: MessageVoteContents{
				BlockHash: newBlock.Hash(),
			},
		}, validators)
		synctest.Wait()
		hs.Stop()
	})
}

func TestHotstuff_ViewTimer_TriggersViewChangeAfterTau(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		server.EXPECT().GetLocalId().AnyTimes()
		server.EXPECT().GetPeers().AnyTimes()

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
			2: 1,
		})
		require.NoError(t, err)

		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()
		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateLineup().Return(lineup).AnyTimes()

		gossip := broadcast.NewMockChannel[Message](ctrl)
		gossip.EXPECT().Register(gomock.Any()).AnyTimes()
		gossip.EXPECT().Unregister(gomock.Any()).AnyTimes()
		gossip.EXPECT().Broadcast(gomock.Any()).AnyTimes()

		Factory := &Factory{
			getGossipFunction: func(p2p.Server) broadcast.Channel[Message] {
				return gossip
			},
			ViewLimit: 2,
		}
		hs := Factory.NewActive(server, *committee, 1, source).(*Hotstuff)
		synctest.Wait()
		time.Sleep(hs.tau)
		synctest.Wait()
		require.Equal(t, uint64(2), hs.curView)
		hs.Stop()
	})
}

func TestHotStuff_ViewTimer_DoesNothingIfRenderedObsolete(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		server.EXPECT().GetLocalId().AnyTimes()
		server.EXPECT().GetPeers().AnyTimes()

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
			2: 1,
		})
		require.NoError(t, err)

		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()
		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateLineup().Return(lineup).AnyTimes()

		gossip := broadcast.NewMockChannel[Message](ctrl)
		gossip.EXPECT().Register(gomock.Any()).AnyTimes()
		gossip.EXPECT().Unregister(gomock.Any()).AnyTimes()
		gossip.EXPECT().Broadcast(gomock.Any()).AnyTimes()

		Factory := &Factory{
			getGossipFunction: func(p2p.Server) broadcast.Channel[Message] {
				return gossip
			},
		}
		hs := Factory.NewActive(server, *committee, 1, source).(*Hotstuff)
		synctest.Wait()
		time.Sleep(hs.tau / 2)
		hs.stateMutex.Lock()
		// Manually advance view to render timer obsolete.
		hs.curView = 10
		hs.stateMutex.Unlock()
		time.Sleep(hs.tau - hs.tau/2)
		hs.stateMutex.Lock()
		require.NotEqual(t, uint64(2), hs.curView)
		hs.stateMutex.Unlock()
		hs.Stop()
	})
}

func TestHotStuff_ProposeTimer_TriggersViewChangeAfter3Delta(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		server := p2p.NewMockServer(ctrl)
		server.EXPECT().RegisterMessageHandler(gomock.Any()).AnyTimes()
		server.EXPECT().GetLocalId().AnyTimes()
		server.EXPECT().GetPeers().AnyTimes()

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
			2: 1,
		})
		require.NoError(t, err)

		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()
		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateLineup().Return(lineup).AnyTimes()

		factory := &Factory{}
		hs := factory.NewActive(server, *committee, 2, source).(*Hotstuff)
		synctest.Wait()
		// Artifically advance to a later view in order to demonstrate behavior with
		// old bestQC.
		hs.stateMutex.Lock()
		hs.advanceView(7)
		require.Equal(t, uint64(7), hs.curView)
		hs.stateMutex.Unlock()
		hs.fakeHandleMessage(Message{
			Signature: 1,
			Type:      NewView,
			View:      7,
			Contents: MessageNewViewContents{
				HighQC: certificate{
					view:       0,
					blockHash:  genesisBlock(committee).Hash(),
					signatures: getFullVoteCounter(committee),
				},
			}},
		)
		time.Sleep(3 * hs.delta)
		synctest.Wait()
		hs.stateMutex.Lock()
		// After 3 delta, the propose timer should have triggered but view hasn't
		// changed yet (tau = 7 delta, so we're still in view 7)
		require.Equal(t, uint64(7), hs.curView)
		hs.stateMutex.Unlock()
		// Now wait for tau to expire and verify view changes
		time.Sleep(hs.tau)
		synctest.Wait()
		hs.stateMutex.Lock()
		require.Equal(t, uint64(8), hs.curView)
		hs.stateMutex.Unlock()
		hs.Stop()
	})
}

func TestHotstuff_StopIsIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		network := p2p.NewNetwork()
		server, err := network.NewServer(p2p.PeerId("server"))
		require.NoError(t, err)

		committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{
			1: 1,
		})
		require.NoError(t, err)

		lineup := txpool.NewMockLineup(ctrl)
		lineup.EXPECT().All().Return([]types.Transaction{}).AnyTimes()

		source := consensus.NewMockTransactionProvider(ctrl)
		source.EXPECT().GetCandidateLineup().Return(lineup).AnyTimes()

		factory := &Factory{}
		hs := factory.NewActive(server, *committee, 1, source).(*Hotstuff)

		// Call Stop multiple times and ensure no panic or error
		require.NotPanics(t, func() {
			hs.Stop()
			hs.Stop()
			hs.Stop()
		})
	})
}

// An auxiliary method to fake-handle messages with proper locking in tests.
func (h *Hotstuff) fakeHandleMessage(msg Message) {
	h.stateMutex.Lock()
	defer h.stateMutex.Unlock()
	h.handleMessage(msg)
}
