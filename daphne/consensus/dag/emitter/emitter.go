package emitter

import (
	"log/slog"
	"time"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

const (
	EmitterInterval = 500 * time.Millisecond
)

type Emitter struct {
	creator model.CreatorId
	dag     *model.Dag
	p2p     p2p.Server
	source  consensus.PayloadSource

	quit chan<- struct{}
	done <-chan struct{}
}

func NewEmitter(
	creator model.CreatorId,
	p2p p2p.Server,
	dag *model.Dag,
	source consensus.PayloadSource,
) *Emitter {
	res := &Emitter{
		creator: creator,
		dag:     dag,
		p2p:     p2p,
		source:  source,
	}

	quit := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(EmitterInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				res.emit()
			case <-quit:
				return
			}
		}
	}()

	res.quit = quit
	res.done = done

	return res
}

func (e *Emitter) emit() {
	transactions := e.source.GetCandidateTransactions()
	event := e.createEvent(transactions)

	msg := p2p.Message{
		Code:    p2p.MessageCode_DagConsensus_NewEvent,
		Payload: event,
	}

	slog.Info("Emit! ", "event", event)

	for _, peer := range e.p2p.GetPeers() {
		if err := e.p2p.SendMessage(peer, msg); err != nil {
			slog.Error("Failed to send event", "peer", peer, "error", err)
		}
	}
}

func (e *Emitter) createEvent(
	payload []types.Transaction,
) model.EventMessage {
	tips := e.dag.GetTips()
	slog.Info("Tips", "tips", tips, "dag", e.dag)

	parents := []model.EventId{}
	if _, found := tips[e.creator]; found {
		parents = []model.EventId{tips[e.creator].EventId()}
		for creator, tip := range tips {
			if creator != e.creator {
				parents = append(parents, tip.EventId())
			}
		}
	}

	event := model.EventMessage{
		Creator: e.creator,
		Parents: parents,
		Payload: payload,
	}
	return event
}
