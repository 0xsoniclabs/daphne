package dag

import (
	"log/slog"
	"slices"
	"sync"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/emitter"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/p2p"
	"github.com/0xsoniclabs/daphne/daphne/types"
)

type Verdict int

const (
	VerdictYes Verdict = iota
	VerdictNo
	VerdictUndecided
)

type Algorithm struct {
	Creator         model.CreatorId
	Committee       map[model.CreatorId]uint32 // Committee for the DAG consensus
	LayeringFactory layering.LayeringFactory
}

func (a Algorithm) NewActive(server p2p.Server, source consensus.PayloadSource) consensus.Consensus {
	return newActive(server, source, &a)
}

func (a Algorithm) NewPassive(server p2p.Server) consensus.Consensus {
	return newPassive(server, a.Committee, a.LayeringFactory)
}

type dagConsensus struct {
	dag            *model.Dag
	p2p            p2p.Server
	broadcastMutex sync.Mutex

	layering         layering.Layering
	leaderCandidates []*model.Event // List of candidate events
	lastDecided      *model.Event

	emitter *emitter.Emitter

	listeners []consensus.BundleListener

	knownSeenEvents map[p2p.PeerId]map[model.EventId]struct{} // Track seen events by peer ID and event number
}

func newActive(server p2p.Server, source consensus.PayloadSource, config *Algorithm) *dagConsensus {
	res := newPassive(server, config.Committee, config.LayeringFactory)
	// start the emitter
	res.emitter = emitter.NewEmitter(
		config.Creator,
		server,
		res.dag,
		source,
	)

	return res
}

func newPassive(server p2p.Server, committee map[model.CreatorId]uint32, layeringFactory layering.LayeringFactory) *dagConsensus {
	dag := model.NewDag()
	res := &dagConsensus{
		dag:             dag,
		p2p:             server,
		knownSeenEvents: make(map[p2p.PeerId]map[model.EventId]struct{}),
		layering:        layeringFactory.NewLayering(dag, committee),
	}
	res.p2p.RegisterMessageHandler(messageHandlerAdapter{c: res})

	return res
}

func (d *dagConsensus) handleMessage(_ p2p.PeerId, msg p2p.Message) {
	if msg.Code != p2p.MessageCode_DagConsensus_NewEvent {
		return
	}
	eventMessage, ok := msg.Payload.(model.EventMessage)
	if !ok {
		return
	}
	connected := d.dag.AddEvent(eventMessage)

	for _, event := range connected {
		d.broadcast(event.ToEventMessage())
	}

	for _, event := range connected {
		isCandidate, err := d.layering.IsCandidate(event)
		if err != nil {
			slog.Error("Failed to process incoming event", "error", err, "eventId", event.EventId())
			return
		}
		if isCandidate {
			d.leaderCandidates = append(d.leaderCandidates, event)
		}
	}

	newLeaders := []*model.Event{}
	d.leaderCandidates = slices.DeleteFunc(
		d.leaderCandidates,
		func(candidate *model.Event) bool {
			isLeader, err := d.layering.IsLeader(candidate)
			if err != nil {
				slog.Error("Failed to check if event is a leader", "error", err, "eventId", candidate.EventId())
				panic("missing proper error propagation")
			}
			switch isLeader {
			case layering.VerdictYes:
				newLeaders = append(newLeaders, candidate)
				return true
			case layering.VerdictNo:
				return true
			case layering.VerdictUndecided:
				return false
			}
			return false
		},
	)

	// Sort leaders based on the layering algorithm.
	newLeaders, err := d.layering.SortLeaders(newLeaders)
	if err != nil {
		slog.Error("Failed to sort leaders", "error", err)
		return
	}

	covered := d.dag.GetClosure(d.lastDecided)
	for _, leader := range newLeaders {
		newCovered := d.dag.GetClosure(leader)
		delta := slices.DeleteFunc(
			slices.Clone(newCovered),
			func(e *model.Event) bool {
				return slices.ContainsFunc(covered, func(a *model.Event) bool {
					return a.EventId() == e.EventId()
				})
			},
		)
		d.processConfirmedEvents(delta)
		d.lastDecided = leader
		covered = newCovered
	}
}

func (d *dagConsensus) RegisterListener(listener consensus.BundleListener) {
	if listener != nil {
		d.listeners = append(d.listeners, listener)
	}
}

func (d *dagConsensus) broadcast(event model.EventMessage) {
	for _, peer := range d.p2p.GetPeers() {
		if d.eventSeenByPeer(peer, event) {
			continue
		}
		err := d.p2p.SendMessage(peer, p2p.Message{
			Code:    p2p.MessageCode_DagConsensus_NewEvent,
			Payload: event,
		})
		if err != nil {
			slog.Warn("Failed to send message", "peerId", peer, "error", err)
			continue
		}
	}
}

func (d *dagConsensus) eventSeenByPeer(peer p2p.PeerId, event model.EventMessage) bool {
	d.broadcastMutex.Lock()
	defer d.broadcastMutex.Unlock()
	if d.knownSeenEvents[peer] == nil {
		d.knownSeenEvents[peer] = make(map[model.EventId]struct{})
	}
	if _, seen := d.knownSeenEvents[peer][event.EventId()]; seen {
		return true
	}
	d.knownSeenEvents[peer][event.EventId()] = struct{}{}
	return false
}

func (d *dagConsensus) processConfirmedEvents(events []*model.Event) {
	// Collect the transactions in the events.
	var transactions []types.Transaction
	for _, event := range events {
		transactions = append(transactions, event.Payload...)
	}
	// Create and produce a bundle from the transactions.
	bundle := types.Bundle{
		Transactions: transactions,
	}
	for _, listener := range d.listeners {
		listener.OnNewBundle(bundle)
	}
}

type messageHandlerAdapter struct {
	c *dagConsensus
}

func (m messageHandlerAdapter) HandleMessage(peerId p2p.PeerId, msg p2p.Message) {
	m.c.handleMessage(peerId, msg)
}
