package dag

import (
	"math/rand"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/autocracy"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering/lachesis"
	"github.com/0xsoniclabs/daphne/daphne/generic"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDagConsensus_ThreeAutocracyNodes_ConsistentlyLinearizesTransactions(t *testing.T) {
	testDagConsensus_ThreeNodes_ConsistentlyLinearizesTransactions(t, autocracy.Factory{CandidateFrequency: 3})
}

func TestDagConsensus_ThreeLachesisNodes_ConsistentlyLinearizesTransactions(t *testing.T) {
	testDagConsensus_ThreeNodes_ConsistentlyLinearizesTransactions(t, lachesis.Factory{})
}

func testDagConsensus_ThreeNodes_ConsistentlyLinearizesTransactions(t *testing.T, layeringFactory layering.Factory) {
	ctrl := gomock.NewController(t)

	committee, err := consensus.NewCommittee(map[consensus.ValidatorId]uint32{1: 1, 2: 1})
	require.NoError(t, err)

	consensusConfig := Factory{
		EmitInterval:    generic.DefaultEmitInterval,
		Creator:         1,
		Committee:       committee,
		LayeringFactory: layeringFactory,
	}
	activeConfig := consensusConfig
	activeConfig.Creator = 2
	// Creator is irrelevant for a passive instance.
	passiveConfig := consensusConfig

	active1Rand := rand.New(rand.NewSource(42))
	active1EmittedTransactions := []types.Transaction{}
	active1TxSource := consensus.NewMockTransactionProvider(ctrl)
	active1TxSource.EXPECT().GetCandidateTransactions().DoAndReturn(func() []types.Transaction {
		txs := slices.Repeat([]types.Transaction{{From: types.Address(active1Rand.Int())}}, active1Rand.Intn(3))
		active1EmittedTransactions = append(active1EmittedTransactions, txs...)
		return txs
	}).AnyTimes()

	active2Rand := rand.New(rand.NewSource(43))
	active2EmittedTransactions := []types.Transaction{}
	active2TxSource := consensus.NewMockTransactionProvider(ctrl)
	active2TxSource.EXPECT().GetCandidateTransactions().DoAndReturn(func() []types.Transaction {
		txs := slices.Repeat([]types.Transaction{{From: types.Address(active2Rand.Int())}}, active2Rand.Intn(3))
		active2EmittedTransactions = append(active2EmittedTransactions, txs...)
		return txs
	}).AnyTimes()

	network := p2p.NewNetwork()
	server1, _ := network.NewServer(p2p.PeerId("active1"))
	server2, _ := network.NewServer(p2p.PeerId("active2"))
	server3, _ := network.NewServer(p2p.PeerId("passive"))

	listenerActive1 := &testListener{}
	listenerActive2 := &testListener{}
	listenerPassive := &testListener{}

	synctest.Test(t, func(t *testing.T) {
		active1 := consensusConfig.NewActive(server1, active1TxSource)
		active2 := activeConfig.NewActive(server2, active2TxSource)
		passive := passiveConfig.NewPassive(server3)
		defer active1.Stop()
		defer active2.Stop()

		active1.RegisterListener(listenerActive1)
		active2.RegisterListener(listenerActive2)
		passive.RegisterListener(listenerPassive)

		time.Sleep(30 * time.Second)
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
		active1EmittedTransactions[:4*len(active1EmittedTransactions)/5],
	)

	// The linearization should be consistent among all nodes.
	require.Equal(t, listenerActive1.linearizedTransactions, listenerActive2.linearizedTransactions)
	require.Equal(t, listenerActive1.linearizedTransactions, listenerPassive.linearizedTransactions)
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
