package lachesis

import (
	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

type Factory struct {
}

func (af Factory) NewLayering(
	committee *consensus.Committee,
) layering.Layering {
	return &Lachesis{}
}

type Lachesis struct {
}

func (l *Lachesis) IsCandidate(event *model.Event) bool {
	return false
}

func (l *Lachesis) IsLeader(dag *model.Dag, event *model.Event) layering.Verdict {
	return layering.VerdictNo
}

func (l *Lachesis) SortLeaders(dag *model.Dag, leaders []*model.Event) []*model.Event {
	return leaders
}
