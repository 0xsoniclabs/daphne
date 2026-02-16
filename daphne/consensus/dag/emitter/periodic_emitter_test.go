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

package emitter

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPeriodicEmitterFactory_IsAnEmitterFactoryImplementation(t *testing.T) {
	var _ Factory = PeriodicEmitterFactory{}
}

func TestPeriodicEmitterFactory_String_ProducesReadableSummary(t *testing.T) {
	factory := PeriodicEmitterFactory{Interval: 150 * time.Millisecond}
	require.Equal(t, "periodic_150ms", factory.String())
}

func TestPeriodicEmitter_IsAnEmitterImplementation(t *testing.T) {
	var _ Emitter = &PeriodicEmitter{}
}

func TestPeriodicEmitter_NewEmitter_TriggersEmissionsPeriodicallyAndStops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)

		testInterval := 10 * time.Millisecond
		const numEmissions = 5

		dag := model.NewMockDag(ctrl)
		dag.EXPECT().GetHeads().Times(numEmissions)

		channel := NewMockChannel(ctrl)
		channel.EXPECT().Emit(gomock.Any()).Times(numEmissions)

		emitter := PeriodicEmitterFactory{Interval: testInterval}.NewEmitter(channel, dag, 0, nil)
		time.Sleep(testInterval * numEmissions)

		emitter.Stop()
	})
}
