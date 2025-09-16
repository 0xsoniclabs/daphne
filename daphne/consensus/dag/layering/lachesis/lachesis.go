package lachesis

import (
	"log/slog"
	"maps"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

type Factory struct {
}

func (af Factory) NewLayering(
	committee *consensus.Committee,
) layering.Layering {
	return &Lachesis{
		lamportTimestampCache: make(map[model.EventId]int),
		frameCache:            make(map[model.EventId]int),
		committee:             committee,
	}
}

type Lachesis struct {
	committee             *consensus.Committee
	frameCache            map[model.EventId]int
	lamportTimestampCache map[model.EventId]int
}

func (l *Lachesis) IsCandidate(event *model.Event) bool {
	// All genesis events are considered frame-1 candidates.
	if event.IsGenesis() {
		return true
	}

	return l.eventFrame(event) != l.eventFrame(event.SelfParent())
}

func (l *Lachesis) eventFrame(event *model.Event) int {
	if frame, ok := l.frameCache[event.EventId()]; ok {
		return frame
	}

	if event.IsGenesis() {
		l.frameCache[event.EventId()] = 1
		return 1
	}

	frame := 1
	for _, parent := range event.Parents() {
		frame = max(frame, l.eventFrame(parent))
	}

	closure := slices.Collect(maps.Keys(event.GetClosure()))

	// Find all ancestor events that are candidates.
	highestObservedFrameCandidates := slices.DeleteFunc(closure, func(e *model.Event) bool {
		return l.eventFrame(e) != frame && !l.IsCandidate(e)
	})

	if l.stronglyReachesQuorum(event, highestObservedFrameCandidates) {
		frame++
	}

	l.frameCache[event.EventId()] = frame
	return frame
}

func (l *Lachesis) stronglyReachesQuorum(event *model.Event, bases []*model.Event) bool {
	voteCounter := consensus.NewVoteCounter(l.committee)

	for _, base := range bases {
		if l.stronglyReaches(event, base) {
			if err := voteCounter.Vote(base.Creator()); err != nil {
				slog.Warn("Failed to register vote", "err", err)
				return false
			}
		}
	}

	return voteCounter.IsQuorumReached()
}

func (l *Lachesis) stronglyReaches(source, target *model.Event) bool {
	// An event strongly reaches another event if it can reach more than 2/3 of the target's validators.
	voteCounter := consensus.NewVoteCounter(l.committee)

	return voteCounter.IsQuorumReached()
}

func (l *Lachesis) IsLeader(dag *model.Dag, event *model.Event) layering.Verdict {
	return layering.VerdictNo
}

func (l *Lachesis) SortLeaders(dag *model.Dag, leaders []*model.Event) []*model.Event {
	return leaders
}

func (l *Lachesis) lamportTimestamp(event *model.Event) int {
	if ts, ok := l.lamportTimestampCache[event.EventId()]; ok {
		return ts
	}

	if event.IsGenesis() {
		l.lamportTimestampCache[event.EventId()] = 1
		return 1
	}

	maxTs := int(1)
	for _, parent := range event.Parents() {
		parentTs := l.lamportTimestamp(parent)
		maxTs = max(maxTs, parentTs+1)
	}

	l.lamportTimestampCache[event.EventId()] = maxTs
	return maxTs
}
