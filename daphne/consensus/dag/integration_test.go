package dag

import (
	"math/rand"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/autocracy"
	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDagConsensus_ThreeNodes_ConsistentlyLinearizesTransactions(t *testing.T) {
	ctrl := gomock.NewController(t)

	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{1: 1, 2: 1})
	require.NoError(t, err)

	layeringFactory := autocracy.Factory{CandidateFrequency: 3}
	autocratConfig := Factory{
		EmitInterval:    generic.DefaultEmitInterval,
		Creator:         1,
		Committee:       committee,
		LayeringFactory: layeringFactory,
	}
	activeConfig := autocratConfig
	activeConfig.Creator = 2
	// Creator is irrelevant for a passive instance.
	passiveConfig := autocratConfig

	autocratRand := rand.New(rand.NewSource(42))
	autocratEmittedTransactions := []types.Transaction{}
	autocratTxSource := consensus.NewMockTransactionProvider(ctrl)
	autocratTxSource.EXPECT().GetCandidateTransactions().DoAndReturn(func() []types.Transaction {
		txs := slices.Repeat([]types.Transaction{{From: types.Address(autocratRand.Int())}}, autocratRand.Intn(3))
		autocratEmittedTransactions = append(autocratEmittedTransactions, txs...)
		return txs
	}).AnyTimes()

	activeRand := rand.New(rand.NewSource(43))
	activeEmittedTransactions := []types.Transaction{}
	activeTxSource := consensus.NewMockTransactionProvider(ctrl)
	activeTxSource.EXPECT().GetCandidateTransactions().DoAndReturn(func() []types.Transaction {
		txs := slices.Repeat([]types.Transaction{{From: types.Address(activeRand.Int())}}, activeRand.Intn(3))
		activeEmittedTransactions = append(activeEmittedTransactions, txs...)
		return txs
	}).AnyTimes()

	network := p2p.NewNetwork()
	server1, _ := network.NewServer(p2p.PeerId("autocrat"))
	server2, _ := network.NewServer(p2p.PeerId("active"))
	server3, _ := network.NewServer(p2p.PeerId("passive"))

	listenerAutocrat := &testListener{}
	listenerActive := &testListener{}
	listenerPassive := &testListener{}

	synctest.Test(t, func(t *testing.T) {
		autocrat := autocratConfig.NewActive(server1, autocratTxSource)
		active := activeConfig.NewActive(server2, activeTxSource)
		passive := passiveConfig.NewPassive(server3)
		defer autocrat.Stop()
		defer active.Stop()

		autocrat.RegisterListener(listenerAutocrat)
		active.RegisterListener(listenerActive)
		passive.RegisterListener(listenerPassive)

		time.Sleep(30 * time.Second)
	})

	// Expect at least ~80% of all emitted txs from both active nodes to be linearized.
	require.Subset(
		t,
		listenerAutocrat.linearizedTransactions,
		autocratEmittedTransactions[:4*len(autocratEmittedTransactions)/5],
	)
	require.Subset(
		t,
		listenerAutocrat.linearizedTransactions,
		activeEmittedTransactions[:4*len(activeEmittedTransactions)/5],
	)

	// The linearization should be consistent among all nodes.
	require.Equal(t, listenerAutocrat.linearizedTransactions, listenerActive.linearizedTransactions)
	require.Equal(t, listenerAutocrat.linearizedTransactions, listenerPassive.linearizedTransactions)
}

type testListener struct {
	txMutex                sync.Mutex
	linearizedTransactions []types.Transaction
}

func (t *testListener) OnNewBundle(bundle types.Bundle) {
	t.txMutex.Lock()
	defer t.txMutex.Unlock()
	t.linearizedTransactions = append(t.linearizedTransactions, bundle.Transactions...)
}
