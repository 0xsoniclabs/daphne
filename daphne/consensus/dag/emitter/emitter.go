package emitter

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

type Channel interface {
	Emit(dagHeadsSnapshot map[consensus.ValidatorId]*model.Event)
}

type Factory interface {
	NewEmitter(Channel, model.Dag, consensus.ValidatorId) Emitter
}

type Emitter interface {
	OnDagChange()
	Stop()
}
