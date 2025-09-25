package lachesis

import (
	"maps"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

// GenesisFrame is the frame number assigned to all genesis events. It is the
// lowest possible frame number in a Lachesis-layered DAG.
const GenesisFrame = 1

type Factory struct{}

// NewLayering creates a new [Lachesis] layering instance.
func (f Factory) NewLayering(
	committee *consensus.Committee,
) layering.Layering {
	return &Lachesis{
		frameCache:           make(map[model.EventId]int),
		stronglyReachesCache: make(map[eventHashPair]bool),
		committee:            committee,
	}
}

// Lachesis layers the DAG by organizing events into frames and electing leaders
// among candidate events in each frame.
//
// Frame of an event is defined as one frame higher than the highest frame
// whose candidates quorum is strongly reachable by the event, having genesis
// events in frame 1 by definition.
//
// An event A strongly reaches event B if A can reach (have a path to in the DAG)
// B through a set of events whose creators form quorum of the committee.
//
// Candidates are the first events in their frame by their creator.
//
// Leaders are elected from candidates based on a voting process involving
// strong reachability and quorum from the committee. The voters (candidates
// from higher frames) cast and aggregate votes on whether they strongly reach
// a candidate.
//

type Lachesis struct {
	committee            *consensus.Committee
	frameCache           map[model.EventId]int
	stronglyReachesCache map[eventHashPair]bool
}

// eventHashPair is used to uniquely identify an ordered pair of events.
type eventHashPair struct {
	source model.EventId
	target model.EventId
}

// IsCandidate returns true if the event is first in its frame by its creator.
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

// IsLeader returns the Lachesis verdict for the given event.
// If the event is not a candidate, VerdictNo is returned.
// If the event is a candidate, the leader election process is executed
// for the candidate's frame - if a leader for the frame is elected and it
// is the provided candidate, VerdictYes is returned. If a different leader is
// elected, the verdict is No. If no leader can be elected for the candidate's
// frame with the provided DAG, VerdictUndecided is returned.
// The election process for a frame can only be executed if all previous frames
// have a decided leader, otherwise the verdict is VerdictUndecided.
func (l *Lachesis) IsLeader(dag *model.Dag, candidate *model.Event) layering.Verdict {
	if !l.IsCandidate(candidate) {
		return layering.VerdictNo
	}

	// If at least one of the previous frames does not have a decided leader,
	// the decision for the candidate's frame cannot be made.
	for frame := 1; frame < l.eventFrame(candidate); frame++ {
		if event, _ := l.electLeader(dag, frame); event == nil {
			return layering.VerdictUndecided
		}
	}

	switch event, ruledOut := l.electLeader(dag, l.eventFrame(candidate)); {
	case event == candidate:
		return layering.VerdictYes
	case event == nil && !slices.Contains(ruledOut, candidate):
		return layering.VerdictUndecided
	default:
		return layering.VerdictNo
	}
}

// SortLeaders orders the provided events by their frames, filtering out non-leaders.
// There can not be multiple leaders in the same frame, as the election process
// guarantees that at most one leader can be elected per frame for a fixed DAG.
func (l *Lachesis) SortLeaders(dag *model.Dag, events []*model.Event) []*model.Event {
	leaders := slices.DeleteFunc(events, func(event *model.Event) bool {
		return l.IsLeader(dag, event) != layering.VerdictYes
	})
	slices.SortFunc(leaders, func(a, b *model.Event) int {
		return l.eventFrame(a) - l.eventFrame(b)
	})
	return leaders
}

// electLeader attempts to elect a leader for the provided frame in the given DAG.
// If no leader can be elected with the provided DAG, nil is returned.
// It also returns all events that are decided with a NO verdict during the election process.
func (l *Lachesis) electLeader(dag *model.Dag, frame int) (*model.Event, []*model.Event) {
	heads := dag.GetHeads()
	relevantEvents := map[*model.Event]struct{}{}

	// Collect all events that are relevant for the election in the target frame.
	for _, head := range heads {
		headClosure := head.GetClosure()
		// Events that are in frames lower than the target frame, or are not candidates
		// (only candidates are elected and vote) are irrelevant for the election.
		maps.DeleteFunc(headClosure, func(e *model.Event, _ struct{}) bool {
			return !l.IsCandidate(e) || l.eventFrame(e) < frame
		})
		maps.Insert(relevantEvents, maps.All(headClosure))
	}

	// Gather all the candidates in the target frame.
	candidates := slices.DeleteFunc(slices.Collect(maps.Keys(relevantEvents)), func(e *model.Event) bool {
		return l.eventFrame(e) != frame
	})

	// Attempt to decide events in a deterministic order, based on stake and creator ID.
	// The higher priority candidate events are those with higher stake creators,
	// where creator ID is used as a tiebreaker in case of a tie.
	// In order for a candidate to be elected as a leader, all higher priority
	// candidates must be decided with a "No" verdict. As all candidate decisions
	// are guaranteed to be consistent amongst honest nodes, and the relative order
	// at which the nodes arrive to the decisions can vary, waiting for all the
	// higher priority candidates to be decided negatively ensures the consistent
	// choice of a leader between all the nodes.
	slices.SortFunc(candidates, func(a, b *model.Event) int {
		aStake := l.committee.GetCreatorStake(a.Creator())
		bStake := l.committee.GetCreatorStake(b.Creator())
		// Creator ID is used as a consistent tie-breaker.
		if aStake == bStake {
			return int(a.Creator()) - int(b.Creator())
		}
		return int(bStake) - int(aStake)
	})

	ruledOutCandidates := []*model.Event{}

candidatesLoop:
	for _, candidate := range candidates {
		prevFrameVotes := map[*model.Event]bool{}
		// Candidate events for frames `frame + 1` and upwards are
		// eligible voters for the provided `frame`.
		//
		// The election process for a single candidate consists of two types of rounds:
		// 1. Voting round: Collection of votes from voters from `frame + 1`. The voter
		//   votes positively if it strongly reaches the candidate, negatively otherwise.
		// 2. Aggregation round: Aggregation of votes from the previous round (previous frame voters).
		//
		for voterFrame := l.eventFrame(candidate) + 1; ; voterFrame++ {
			voters := maps.Clone(relevantEvents)
			maps.DeleteFunc(voters, func(e *model.Event, _ struct{}) bool {
				return l.eventFrame(e) != voterFrame
			})
			if len(voters) == 0 {
				// If voters are exhausted and no decision was reached for the
				// (currently) highest priority candidate, decision cannot be
				// made for this frame with the provided DAG.
				break candidatesLoop
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
					yesCounter := consensus.NewVoteCounter(l.committee)
					noCounter := consensus.NewVoteCounter(l.committee)
					for prevFrameVoter, prevFrameVote := range prevFrameVotes {
						if l.stronglyReaches(voter, prevFrameVoter) {
							if prevFrameVote {
								yesCounter.Vote(prevFrameVoter.Creator())
							} else {
								noCounter.Vote(prevFrameVoter.Creator())
							}
						}
					}
					if yesCounter.IsQuorumReached() {
						return candidate, ruledOutCandidates
					} else if noCounter.IsQuorumReached() {
						// This event is no longer the highest priority candidate,
						// stop the voting and election process completely.
						ruledOutCandidates = append(ruledOutCandidates, candidate)
						continue candidatesLoop
					}
					// Any of the two counters can be used to determine a simple majority.
					votes[voter] = yesCounter.IsMajorityReached()
				}
			}
			prevFrameVotes = votes
			// This round of voting did not lead to a decision, continue to the next round.
			// Until all voters are exhausted.
		}
	}

	return nil, ruledOutCandidates
}

// eventFrame computes the frame of an event and memoizes the result.
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

	// Find the highest highestObservedFrame among parents.
	highestObservedFrame := GenesisFrame
	for _, parent := range event.Parents() {
		highestObservedFrame = max(highestObservedFrame, l.eventFrame(parent))
	}

	// Find all ancestor events that are candidates and have same frame as the highest observed frame.
	closure := event.GetClosure()
	delete(closure, event)
	highestObservedFrameCandidates := slices.DeleteFunc(slices.Collect(maps.Keys(closure)), func(e *model.Event) bool {
		return l.eventFrame(e) != highestObservedFrame || !l.IsCandidate(e)
	})
	if l.stronglyReachesQuorum(event, highestObservedFrameCandidates) {
		highestObservedFrame++
	}

	l.frameCache[event.EventId()] = highestObservedFrame
	return highestObservedFrame
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
	stronglyReachesCacheKey := eventHashPair{source.EventId(), target.EventId()}
	if stronglyReaches, ok := l.stronglyReachesCache[stronglyReachesCacheKey]; ok {
		return stronglyReaches
	}
	// Gather the transit events - i.e. the events from source's closure that
	// observe the target (including the source and the target).
	transitEvents := source.GetClosure()
	maps.DeleteFunc(transitEvents, func(e *model.Event, _ struct{}) bool {
		_, reachesTarget := e.GetClosure()[target]
		return !reachesTarget
	})

	voteCounter := consensus.NewVoteCounter(l.committee)
	for e := range transitEvents {
		voteCounter.Vote(e.Creator())
	}

	stronglyReaches := voteCounter.IsQuorumReached()
	l.stronglyReachesCache[stronglyReachesCacheKey] = stronglyReaches
	return stronglyReaches
}
