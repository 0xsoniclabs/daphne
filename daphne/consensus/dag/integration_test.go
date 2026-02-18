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

// Copyright 2026 Sonic Labs
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

package dag

import (
	"math/rand"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/emitter"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/autocracy"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/moira"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/txpool"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDagConsensus_ThreeAutocracyNodes_ConsistentlyLinearizesTransactions(t *testing.T) {
	testDagConsensus_ThreeNodes_ConsistentlyLinearizesTransactions(t, autocracy.Factory{CandidateFrequency: 3})
}

func TestDagConsensus_ThreeLachesisNodes_ConsistentlyLinearizesTransactions(t *testing.T) {
	testDagConsensus_ThreeNodes_ConsistentlyLinearizesTransactions(t, moira.LachesisFactory{})
}

func TestDagConsensus_ThreeAtroposNodes_ConsistentlyLinearizesTransactions(t *testing.T) {
	testDagConsensus_ThreeNodes_ConsistentlyLinearizesTransactions(t, moira.AtroposFactory{})
}

func testDagConsensus_ThreeNodes_ConsistentlyLinearizesTransactions(t *testing.T, layeringFactory layering.Factory) {
	ctrl := gomock.NewController(t)

	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{1: 1, 2: 1})
	require.NoError(t, err)

	consensusConfig := Factory[payload.Transactions]{
		EmitterFactory:         &emitter.PeriodicEmitterFactory{Interval: testEmitInterval},
		LayeringFactory:        layeringFactory,
		PayloadProtocolFactory: payload.RawProtocolFactory{},
	}

	active1Rand := rand.New(rand.NewSource(42))
	active1EmittedTransactions := []types.Transaction{}
	active1Lineup := txpool.NewMockLineup(ctrl)
	active1Lineup.EXPECT().All().DoAndReturn(func() []types.Transaction {
		txs := slices.Repeat([]types.Transaction{{From: types.Address(active1Rand.Int())}}, active1Rand.Intn(3))
		active1EmittedTransactions = append(active1EmittedTransactions, txs...)
		return txs
	}).AnyTimes()
	active1TxSource := consensus.NewMockTransactionProvider(ctrl)
	active1TxSource.EXPECT().GetCandidateLineup().Return(active1Lineup).AnyTimes()

	active2Rand := rand.New(rand.NewSource(43))
	active2EmittedTransactions := []types.Transaction{}
	active2Lineup := txpool.NewMockLineup(ctrl)
	active2Lineup.EXPECT().All().DoAndReturn(func() []types.Transaction {
		txs := slices.Repeat([]types.Transaction{{From: types.Address(active2Rand.Int())}}, active2Rand.Intn(3))
		active2EmittedTransactions = append(active2EmittedTransactions, txs...)
		return txs
	}).AnyTimes()
	active2TxSource := consensus.NewMockTransactionProvider(ctrl)
	active2TxSource.EXPECT().GetCandidateLineup().Return(active2Lineup).AnyTimes()

	network := p2p.NewNetwork()
	server1, _ := network.NewServer(p2p.PeerId("active1"))
	server2, _ := network.NewServer(p2p.PeerId("active2"))
	server3, _ := network.NewServer(p2p.PeerId("passive"))

	listenerActive1 := &testListener{}
	listenerActive2 := &testListener{}
	listenerPassive := &testListener{}

	synctest.Test(t, func(t *testing.T) {
		active1 := consensusConfig.NewActive(server1, *committee, 1, active1TxSource)
		active2 := consensusConfig.NewActive(server2, *committee, 2, active2TxSource)
		passive := consensusConfig.NewPassive(server3, *committee)

		active1.RegisterListener(listenerActive1)
		active2.RegisterListener(listenerActive2)
		passive.RegisterListener(listenerPassive)

		time.Sleep(30 * time.Second)

		active1.Stop()
		active2.Stop()
		passive.Stop()
	})

	// Expect at least ~80% of all emitted txs from both active nodes to be linearized.
	require.Subset(
		t,
		listenerActive1.linearizedTransactions,
		active1EmittedTransactions[:4*len(active1EmittedTransactions)/5],
	)
	require.Subset(
		t,
		listenerActive1.linearizedTransactions,
		active2EmittedTransactions[:4*len(active2EmittedTransactions)/5],
	)

	minLen := min(len(listenerActive1.linearizedTransactions), len(listenerActive2.linearizedTransactions), len(listenerPassive.linearizedTransactions))
	require.Equal(t,
		listenerActive1.linearizedTransactions[:minLen],
		listenerActive2.linearizedTransactions[:minLen],
	)
	require.Equal(t,
		listenerActive1.linearizedTransactions[:minLen],
		listenerPassive.linearizedTransactions[:minLen],
	)
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
