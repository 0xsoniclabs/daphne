package lachesis

import (
	"cmp"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

// GenesisFrame is the frame number assigned to all genesis events. It is the
// lowest possible frame number in a Lachesis-layered DAG.
const GenesisFrame = 1

type Factory struct{}

// NewLayering creates a new [Lachesis] layering instance.
func (f Factory) NewLayering(
	committee *consensus.Committee,
) layering.Layering {
	return newLachesis(committee)
}

// Lachesis layers the DAG by organizing events into frames and electing leaders
// among candidate events in each frame.
//
// Frame of an event is defined as one frame higher than the highest frame
// whose quorum of candidates is strongly reachable by the event, having genesis
// events in frame 1 by definition.
//
// An event A strongly reaches event B if A can reach B (meaning A is a descendent
// of B and observes no forks by B's creator) through a set of events whose
// creators form a quorum, while not observing any forks by B's creator.
//
// A fork is a pair of events created by the same creator where neither event
// can reach the other.
// DISCLAIMER: Fork detection is not implemented in the current version of the code.
// TODO(#399): implement fork detection
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
	electedLeadersCache  map[int]*model.Event
	lowestUndecidedFrame int
}

func newLachesis(committee *consensus.Committee) *Lachesis {
	return &Lachesis{
		frameCache:           make(map[model.EventId]int),
		stronglyReachesCache: make(map[eventHashPair]bool),
		committee:            committee,
		electedLeadersCache:  make(map[int]*model.Event),
		lowestUndecidedFrame: 1,
	}
}

// eventHashPair is used to uniquely identify an ordered pair of events.
type eventHashPair struct {
	source model.EventId
	target model.EventId
}

// IsCandidate returns true if the event is first in its frame by its creator.
func (l *Lachesis) IsCandidate(event *model.Event) bool {
	if event == nil || !slices.Contains(l.committee.Validators(), event.Creator()) {
		return false
	}
	// All genesis events are frame 1 candidates by definition.
	if event.IsGenesis() {
		return true
	}

	// From definition, a non-genesis event is a candidate if it has a different
	// frame than its self-parent.
	return l.getEventFrame(event.SelfParent()) != l.getEventFrame(event)
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
	// The starting point for the loop is the lowest undecided frame, which
	// acts as a checkpoint under the assumption that the provided dag is
	// monotonically increasing with each call.
	for frame := l.lowestUndecidedFrame; frame < l.getEventFrame(candidate); frame++ {
		if event, _ := l.electLeader(dag, frame); event == nil {
			return layering.VerdictUndecided
		}
	}

	switch event, eventsRuledOutAsLeaders := l.electLeader(dag, l.getEventFrame(candidate)); {
	case event == candidate:
		return layering.VerdictYes
	case event == nil && !slices.Contains(eventsRuledOutAsLeaders, candidate):
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
		return l.getEventFrame(a) - l.getEventFrame(b)
	})
	return leaders
}

// electLeader attempts to elect a leader for the provided frame in the given DAG.
// If no leader can be elected with the provided DAG, nil is returned.
// It also returns all events that are decided with a NO verdict during the election process.
func (l *Lachesis) electLeader(dag *model.Dag, frame int) (*model.Event, []*model.Event) {
	if electedLeader, alreadyElected := l.electedLeadersCache[frame]; alreadyElected {
		return electedLeader, nil
	}

	heads := dag.GetHeads()
	relevantEvents := sets.Empty[*model.Event]()

	// Collect all events that are relevant for the election in the target frame.
	// An event is relevant if it is a candidate in the target frame or
	// it is a candidate in a higher frame (i.e. it is an eligible voter).
	for _, head := range heads {
		head.TraverseClosure(model.WrapEventVisitor(
			func(e *model.Event) model.VisitResult {
				// Events that are in frames lower than the target frame, or are not candidates
				// (only candidates are elected and vote) are irrelevant for the election.
				if l.getEventFrame(e) < frame || relevantEvents.Contains(e) {
					return model.Visit_Prune
				}
				if l.IsCandidate(e) {
					relevantEvents.Add(e)
				}
				return model.Visit_Descent
			}),
		)
	}

	// Gather all the candidates in the target frame.
	candidates := slices.DeleteFunc(slices.Collect(relevantEvents.All()), func(e *model.Event) bool {
		return l.getEventFrame(e) != frame
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
		aStake := l.committee.GetValidatorStake(a.Creator())
		bStake := l.committee.GetValidatorStake(b.Creator())
		// Creator ID is used as a consistent tie-breaker.
		if aStake == bStake {
			return cmp.Compare(a.Creator(), b.Creator())
		}
		return cmp.Compare(bStake, aStake)

	})

	// Collect voters for each frame above the target frame.
	// Terminate when no more voters are found for a frame.
	votersForFrame := map[int]sets.Set[*model.Event]{}
	for voterFrame := frame + 1; ; voterFrame++ {
		voters := relevantEvents.Clone()
		voters.RemoveFunc(func(e *model.Event) bool {
			return l.getEventFrame(e) != voterFrame
		})
		if voters.IsEmpty() {
			break
		}
		votersForFrame[voterFrame] = voters
	}

	ruledOutCandidates := []*model.Event{}

candidatesLoop:
	for _, candidate := range candidates {
		// Candidate events for frames `frame + 1` and upwards are
		// eligible voters for the provided `frame`.
		//
		// The election process for a single candidate consists of two types of rounds:
		// 1. Voting round: Collection of votes by voters from `frame + 1`. The voter
		//   votes positively if it strongly reaches the candidate, negatively otherwise.
		// 2. Aggregation round: Aggregation of votes from the previous round (previous frame voters).
		//   All rounds after the first voting round are aggregation rounds. If an
		//   aggregation round produces no decisions, the aggregating voters become the
		//   voters for the next round (next frame), by placing votes based on a simple
		//   majority.

		// Round 1: just collect the votes. A voter votes positively if it
		// strongly reaches the candidate, negatively otherwise.
		votes := map[*model.Event]bool{}
		for voter := range votersForFrame[frame+1].All() {
			votes[voter] = l.stronglyReaches(voter, candidate)
		}
		// Aggregation rounds: If the aggregating voter strongly reaches a quorum
		// of voters from the previous frame, which voted the same for a specific
		// candidate, it makes a YES/NO decision for that candidate.
		prevFrameVotes := votes
		for currentFrame := frame + 2; ; currentFrame++ {
			votes := map[*model.Event]bool{}
			voters := votersForFrame[currentFrame]
			if voters.IsEmpty() {
				// If voters are exhausted and no decision was reached for the
				// (currently) highest priority candidate, decision cannot be
				// made for this frame with the provided DAG.
				break candidatesLoop
			}
			for voter := range voters.All() {
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
				// If the quorum is reached, make a definite decision for the candidate.
				if yesCounter.IsQuorumReached() {
					l.electedLeadersCache[frame] = candidate
					// electLeader will never be invoked again for a frame
					// lower or equal to lowestUndecidedFrame.
					l.lowestUndecidedFrame = frame + 1
					return candidate, ruledOutCandidates
				} else if noCounter.IsQuorumReached() {
					// This event is no longer the highest priority candidate,
					// stop the voting and election process for it completely.
					ruledOutCandidates = append(ruledOutCandidates, candidate)
					continue candidatesLoop
				}
				// If no quorum is reached, the voter places a vote based on simple majority.
				// Any of the two counters can be used to determine this vote.
				votes[voter] = yesCounter.IsMajorityReached()
			}
			prevFrameVotes = votes
		}
	}

	return nil, ruledOutCandidates
}

// getEventFrame computes the frame of an event and memoizes the result.
// The frame of an event is defined as one frame higher than the highest frame
// whose quorum of candidates is strongly reachable by the event.
// The equivalent definition would be that the frame of an event is the highest
// frame of its parents, plus one if and only if it strongly
// reaches a quorum of candidates in that frame.
// All genesis events are by definition in frame 1.
func (l *Lachesis) getEventFrame(event *model.Event) int {
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
		highestObservedFrame = max(highestObservedFrame, l.getEventFrame(parent))
	}

	highestObservedFrameCandidates := []*model.Event{}

	event.TraverseClosure(
		model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			if e == event {
				return model.Visit_Descent
			}
			if l.getEventFrame(e) < highestObservedFrame {
				return model.Visit_Prune
			}
			if l.getEventFrame(e) == highestObservedFrame && l.IsCandidate(e) {
				highestObservedFrameCandidates = append(highestObservedFrameCandidates, e)
			}
			return model.Visit_Descent
		}),
	)

	frame := highestObservedFrame
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
	stronglyReachesCacheKey := eventHashPair{source.EventId(), target.EventId()}
	if stronglyReaches, ok := l.stronglyReachesCache[stronglyReachesCacheKey]; ok {
		return stronglyReaches
	}

	transitEvents := sets.Empty[*model.Event]()

	source.TraverseClosure(
		model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			// Events in frames lower than the target's frame cannot
			// possibly reach the target.
			// Exempt the source event itself from this check to prevent
			// infinite-recursive calls to getEventFrame.
			if source != e && l.getEventFrame(e) < l.getEventFrame(target) {
				return model.Visit_Prune
			}
			if l.reaches(e, target) {
				transitEvents.Add(e)
			}
			return model.Visit_Descent
		}),
	)

	voteCounter := consensus.NewVoteCounter(l.committee)
	for e := range transitEvents.All() {
		voteCounter.Vote(e.Creator())
	}

	stronglyReaches := voteCounter.IsQuorumReached()
	l.stronglyReachesCache[stronglyReachesCacheKey] = stronglyReaches
	return stronglyReaches
}

func (l *Lachesis) reaches(source *model.Event, target *model.Event) bool {
	if source == target {
		return true
	}
	reachesTarget := false

	source.TraverseClosure(
		model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			// Exempt the source event itself from this check to prevent
			// infinite-recursive calls to getEventFrame.
			if source == e {
				return model.Visit_Descent
			}
			if l.getEventFrame(e) < l.getEventFrame(target) {
				return model.Visit_Prune
			}
			if e == target {
				reachesTarget = true
				return model.Visit_Abort
			}
			return model.Visit_Descent
		}),
	)

	return reachesTarget
}
