package lachesis

import (
	"maps"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

const GenesisFrame = 1

type Factory struct{}

func (af Factory) NewLayering(
	committee *consensus.Committee,
) layering.Layering {
	return &Lachesis{
		frameCache: make(map[model.EventId]int),
		committee:  committee,
	}
}

type Lachesis struct {
	committee  *consensus.Committee
	frameCache map[model.EventId]int
}

// An event is a candidate if it is the first event in its frame by its creator.
func (l *Lachesis) IsCandidate(event *model.Event) bool {
	if event == nil || !slices.Contains(l.committee.Creators(), event.Creator()) {
		return false
	}
	// All genesis events are frame 1 candidates by definition.
	if event.IsGenesis() {
		return true
	}

	// From definition, a non-genesis event is a candidate if it has a different
	// frame than its self-parent.
	return l.eventFrame(event.SelfParent()) != l.eventFrame(event)
}

func (l *Lachesis) IsLeader(dag *model.Dag, candidate *model.Event) layering.Verdict {
	if !l.IsCandidate(candidate) {
		return layering.VerdictNo
	}

	// If at least one of the previous frames does not have a decided leader,
	// the decision for the candidate's frame cannot be made.
	for frame := 1; frame < l.eventFrame(candidate); frame++ {
		if l.electLeader(dag, frame) == nil {
			return layering.VerdictUndecided
		}
	}

	switch l.electLeader(dag, l.eventFrame(candidate)) {
	case nil:
		return layering.VerdictUndecided
	case candidate:
		return layering.VerdictYes
	default:
		return layering.VerdictNo
	}
}

func (l *Lachesis) SortLeaders(dag *model.Dag, events []*model.Event) []*model.Event {
	leaders := slices.DeleteFunc(events, func(event *model.Event) bool {
		return l.IsLeader(dag, event) != layering.VerdictYes
	})
	slices.SortFunc(leaders, func(a, b *model.Event) int {
		return l.eventFrame(a) - l.eventFrame(b)
	})
	return leaders
}

func (l *Lachesis) electLeader(dag *model.Dag, frame int) *model.Event {
	heads := dag.GetHeads()
	relevantEvents := map[*model.Event]struct{}{}

	// Collect all events from the dag.
	for _, head := range heads {
		headClosure := head.GetClosure()
		// Events that are in frames lower than the target frame are irrelevant for the election.
		maps.DeleteFunc(headClosure, func(e *model.Event, _ struct{}) bool {
			return l.eventFrame(e) < frame
		})
		maps.Insert(relevantEvents, maps.All(headClosure))
	}

	// Gather all the candidates in the target frame.
	candidates := slices.DeleteFunc(slices.Collect(maps.Keys(relevantEvents)), func(e *model.Event) bool {
		return !l.IsCandidate(e) || l.eventFrame(e) != frame
	})

	// Lachesis uses a deterministic ordering of candidates based on stake and creator ID.
	// The higher priority candidates are those with more stake, and in case of a tie,
	// the creator ID is used as a tiebreaker. In order for a candidate to be elected as a leader,
	// all higher priority candidates must be decided with a "No" verdict. As all candidate decisions
	// are guaranteed to be consistent amongst honest nodes, this enforces a consistent ordering of leaders.
	slices.SortFunc(candidates, func(a, b *model.Event) int {
		aStake := l.committee.GetCreatorStake(a.Creator())
		bStake := l.committee.GetCreatorStake(b.Creator())
		// Creator ID is used as a consistent tie-breaker.
		if aStake == bStake {
			return int(a.Creator()) - int(b.Creator())
		}
		return int(aStake) - int(bStake)
	})

	for _, candidate := range candidates {
		prevFrameVotes := map[*model.Event]bool{}
		// Start voting from candidate event's voterFrame + 1 and upwards.
		for voterFrame := l.eventFrame(candidate) + 1; ; voterFrame++ {
			voters := maps.Clone(relevantEvents)
			maps.DeleteFunc(voters, func(e *model.Event, _ struct{}) bool {
				return l.eventFrame(e) != voterFrame
			})
			if len(voters) == 0 {
				// If voters are exhausted and no decision was reached for the highest priority candidate,
				// decision cannot be made for this frame with the provided DAG.
				return nil
			}

			votes := map[*model.Event]bool{}
			if voterFrame == l.eventFrame(candidate)+1 {
				// In the first voting round, just collect the votes.
				for voter := range voters {
					votes[voter] = l.stronglyReaches(voter, candidate)
				}
			} else {
				// Aggregation round.
				for voter := range voters {
					// If a single voter from this frame strongy reaches a quorum of positive votes from
					// the previous frame, it decides the current
					voteCounter := consensus.NewVoteCounter(l.committee)
					for prevFrameVoter, prevFrameVote := range prevFrameVotes {
						if prevFrameVote && l.stronglyReaches(voter, prevFrameVoter) {
							voteCounter.Vote(prevFrameVoter.Creator())
						}
					}
					if voteCounter.IsQuorumReached() {
						return candidate
					} else if voteCounter.AntiQuorumReached() {
						// We can stop processing this competitor, as it has lost.
						break
					}
					votes[voter] = voteCounter.SimpleMajorityReached()
				}
			}
			prevFrameVotes = votes
			// This round of voting did not lead to a decision, continue to the next round.
			// Until all voters are exhausted.
		}
	}

	return nil
}

// eventFrame computes the frame of an event and memoizes the results.
// The frame of an event is defined as one frame higher than the highest frame
// whose candidates quorum is strongly reachable by the event.
// The equivalent definition would be that the frame of an event is the highest
// frame of its parents, plus one if and only if it strongly
// reaches a quorum of candidates in that frame.
// All genesis events are by definition in frame 1.
func (l *Lachesis) eventFrame(event *model.Event) int {
	if frame, ok := l.frameCache[event.EventId()]; ok {
		return frame
	}

	if event.IsGenesis() {
		l.frameCache[event.EventId()] = GenesisFrame
		return GenesisFrame
	}

	// Find the highest frame among parents.
	frame := GenesisFrame
	for _, parent := range event.Parents() {
		frame = max(frame, l.eventFrame(parent))
	}

	// Find all ancestor events that are candidates and have same frame as the highest observed frame.
	closure := event.GetClosure()
	delete(closure, event)
	highestObservedFrameCandidates := slices.DeleteFunc(slices.Collect(maps.Keys(closure)), func(e *model.Event) bool {
		return l.eventFrame(e) != frame || !l.IsCandidate(e)
	})
	if l.stronglyReachesQuorum(event, highestObservedFrameCandidates) {
		frame++
	}

	l.frameCache[event.EventId()] = frame
	return frame
}

// stronglyReachesQuorum checks if the event strongly reaches a quorum of provided events.
func (l *Lachesis) stronglyReachesQuorum(event *model.Event, bases []*model.Event) bool {
	voteCounter := consensus.NewVoteCounter(l.committee)

	for _, base := range bases {
		if l.stronglyReaches(event, base) {
			voteCounter.Vote(base.Creator())
		}
	}

	return voteCounter.IsQuorumReached()
}

// stronglyReaches checks if an event reaches another event through a supermajority of validators.
func (l *Lachesis) stronglyReaches(source, target *model.Event) bool {
	// Gather the transit events - i.e. the events from source's closure that
	// observe the target (including the source and the target).
	transitEvents := source.GetClosure()
	maps.DeleteFunc(transitEvents, func(e *model.Event, _ struct{}) bool {
		_, seesTarget := e.GetClosure()[target]
		return !seesTarget
	})

	voteCounter := consensus.NewVoteCounter(l.committee)
	for e := range transitEvents {
		voteCounter.Vote(e.Creator())
	}

	return voteCounter.IsQuorumReached()
}
