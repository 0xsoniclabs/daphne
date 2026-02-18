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

package integrationtests

import (
	"testing"
	"testing/synctest"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/central"
	"github.com/0xsoniclabs/daphne/daphne/node"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
	"github.com/stretchr/testify/require"
)

func TestNode_MultiNode_SyncsTransactionPools(t *testing.T) {
	require := require.New(t)

	committee := consensus.NewUniformCommittee(2)

	config := node.NodeConfig{
		Network:   p2p.NewNetwork(),
		Consensus: central.Factory{},
	}

	node1, err := node.NewActiveNode(p2p.PeerId("node1"), *committee, 1, config)
	require.NoError(err)

	node2, err := node.NewActiveNode(p2p.PeerId("node2"), *committee, 2, config)
	require.NoError(err)

	tx := types.Transaction{From: 1}

	synctest.Test(t, func(t *testing.T) {
		rpc1 := node1.GetRpcService()
		require.NoError(rpc1.Send(tx))
		require.True(rpc1.IsPending(tx.Hash()))

		synctest.Wait()

		rpc2 := node2.GetRpcService()
		require.True(rpc2.IsPending(tx.Hash()))
	})
}
