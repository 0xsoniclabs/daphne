package moira

import (
	"cmp"
	"maps"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

type eventRelationFunc func(source, target *model.Event) bool

type Factory struct {
	CandidateLayerRelation eventRelationFunc
	VotingLayerRelation    eventRelationFunc
}

type voterSlot struct {
	frame int
	layer int
}

func (f Factory) String() string {
	return "moira"
}

type Moira struct {
	*Factory
	dag                  model.Dag
	committee            *consensus.Committee
	frameCache           map[model.EventId]int
	voterCache           map[voterSlot]map[*model.Event]bool
	electedLeadersCache  map[int]*model.Event
	lowestUndecidedFrame int
}

func (f Factory) NewLayering(dag model.Dag, committee *consensus.Committee) layering.Layering {
	return NewMoira(&f, dag, committee)
}

func NewMoira(config *Factory, dag model.Dag, committee *consensus.Committee) *Moira {
	return &Moira{
		Factory:              config,
		dag:                  dag,
		committee:            committee,
		frameCache:           make(map[model.EventId]int),
		electedLeadersCache:  make(map[int]*model.Event),
		voterCache:           make(map[voterSlot]map[*model.Event]bool),
		lowestUndecidedFrame: 1,
	}
}

func (b *Moira) IsCandidate(event *model.Event) bool {
	if event == nil || !slices.Contains(b.committee.Validators(), event.Creator()) {
		return false
	}
	// All genesis events are frame 1 candidates by definition.
	if event.IsGenesis() {
		return true
	}

	// From definition, a non-genesis event is a candidate if it has a different
	// frame than its self-parent.
	return b.getEventFrame(event.SelfParent()) != b.getEventFrame(event)
}

func (b *Moira) getEventFrame(event *model.Event) int {
	if frame, ok := b.frameCache[event.EventId()]; ok {
		return frame
	}

	if event.IsGenesis() {
		b.frameCache[event.EventId()] = 1
		return 1
	}

	// Find the highest highestObservedFrame among parents.
	highestObservedFrame := 1
	for _, parent := range event.Parents() {
		highestObservedFrame = max(highestObservedFrame, b.getEventFrame(parent))
	}

	highestObservedFrameCandidates := []*model.Event{}

	event.TraverseClosure(
		model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			if e == event {
				return model.Visit_Descent
			}
			if b.getEventFrame(e) < highestObservedFrame {
				return model.Visit_Prune
			}
			if b.getEventFrame(e) == highestObservedFrame && b.IsCandidate(e) {
				highestObservedFrameCandidates = append(highestObservedFrameCandidates, e)
			}
			return model.Visit_Descent
		}),
	)

	frame := highestObservedFrame
	if b.quorumOfRelations(event, highestObservedFrameCandidates, b.CandidateLayerRelation) {
		frame++
	}

	b.frameCache[event.EventId()] = frame
	return frame
}

func (b *Moira) IsLeader(candidate *model.Event) layering.Verdict {
	if !b.IsCandidate(candidate) {
		return layering.VerdictNo
	}

	// If at least one of the previous frames does not have a decided leader,
	// the decision for the candidate's frame cannot be made.
	// The starting point for the loop is the lowest undecided frame, which
	// acts as a checkpoint under the assumption that the provided dag is
	// monotonically increasing with each call.
	for frame := b.lowestUndecidedFrame; frame < b.getEventFrame(candidate); frame++ {
		if event, _ := b.electLeader(frame); event == nil {
			return layering.VerdictUndecided
		}
	}

	switch event, eventsRuledOutAsLeaders := b.electLeader(b.getEventFrame(candidate)); {
	case event == candidate:
		return layering.VerdictYes
	case event == nil && !slices.Contains(eventsRuledOutAsLeaders, candidate):
		return layering.VerdictUndecided
	default:
		return layering.VerdictNo
	}
}

func (b *Moira) electLeader(frame int) (*model.Event, []*model.Event) {
	if electedLeader, alreadyElected := b.electedLeadersCache[frame]; alreadyElected {
		return electedLeader, nil
	}

	relevantEvents := sets.Empty[*model.Event]()

	// Collect all events that are relevant for the election in the target frame.
	// An event is relevant if it is a candidate in the target frame or
	// it is a candidate in a higher frame (i.e. it is an eligible voter).
	for _, head := range b.dag.GetHeads() {
		head.TraverseClosure(model.WrapEventVisitor(
			func(e *model.Event) model.VisitResult {
				// Events that are in frames lower than the target frame, or are not candidates
				// (only candidates are elected and vote) are irrelevant for the election.
				if b.getEventFrame(e) < frame || relevantEvents.Contains(e) {
					return model.Visit_Prune
				}
				relevantEvents.Add(e)
				return model.Visit_Descent
			}),
		)
	}

	// Gather all the candidates in the target frame.
	candidates := sets.Filter(relevantEvents, func(e *model.Event) bool {
		return b.IsCandidate(e) && b.getEventFrame(e) == frame
	}).ToSlice()

	relevantEvents.Remove(candidates...)

	// Attempt to decide events in a deterministic order, based on stake and creator ID.
	// The higher priority candidate events are those with higher stake creators,
	// where creator ID is used as a tiebreaker in case of a tie.
	// In order for a candidate to be elected as a leader, all higher priority
	// candidates must be decided with a "No" verdict. As all candidate decisions
	// are guaranteed to be consistent amongst honest nodes, and the relative order
	// at which the nodes arrive to the decisions can vary, waiting for all the
	// higher priority candidates to be decided negatively ensures the consistent
	// choice of a leader between all the nodes.
	slices.SortFunc(candidates, func(eventA, eventB *model.Event) int {
		aStake := b.committee.GetValidatorStake(eventA.Creator())
		bStake := b.committee.GetValidatorStake(eventB.Creator())
		// Creator ID is used as a consistent tie-breaker.
		if aStake == bStake {
			return cmp.Compare(eventA.Creator(), eventB.Creator())
		}
		return cmp.Compare(bStake, aStake)

	})

	// Collect voters for each frame above the target frame.
	// Terminate when no more voters are found for a frame.
	votersForLayer := map[int]sets.Set[*model.Event]{}
	for votingLayer := 0; ; votingLayer++ {
		voterSlot := voterSlot{frame: frame, layer: votingLayer}

		if _, exists := b.voterCache[voterSlot]; !exists {
			b.voterCache[voterSlot] = make(map[*model.Event]bool)
		}

		referenceLayer := candidates
		if votingLayer != 0 {
			referenceLayer = votersForLayer[votingLayer-1].ToSlice()
		}
		currentLayer := map[consensus.ValidatorId]*model.Event{}
		order := slices.SortedFunc(relevantEvents.All(), func(e1, e2 *model.Event) int {
			return cmp.Compare(b.getEventFrame(e1), b.getEventFrame(e2))
		})
		for _, event := range order {
			if isVoter, isCached := b.voterCache[voterSlot][event]; isCached {
				if isVoter {
					currentLayer[event.Creator()] = event
				}
				continue
			}
			if competitorVoter, exists := currentLayer[event.Creator()]; exists {
				if competitorVoter.Seq() < event.Seq() {
					b.voterCache[voterSlot][event] = false
					continue
				}
			}
			potentialVoter := b.quorumOfRelations(event, referenceLayer, b.dag.StronglyReaches)
			if potentialVoter {
				currentLayer[event.Creator()] = event
			} else {
				b.voterCache[voterSlot][event] = false
			}
		}
		voters := sets.New(slices.Collect(maps.Values(currentLayer))...)
		sets.Map(voters, func(e *model.Event) *model.Event {
			b.voterCache[voterSlot][e] = true
			return e
		})

		if voters.IsEmpty() {
			break
		}
		votersForLayer[votingLayer] = voters
		relevantEvents.RemoveAll(voters)
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
		for voter := range votersForLayer[0].All() {
			votes[voter] = b.VotingLayerRelation(voter, candidate)
		}
		// Aggregation rounds: If the aggregating voter strongly reaches a quorum
		// of voters from the previous frame, which voted the same for a specific
		// candidate, it makes a YES/NO decision for that candidate.
		prevFrameVotes := votes
		for votingLayer := 1; ; votingLayer++ {
			votes := map[*model.Event]bool{}
			voters := votersForLayer[votingLayer]
			if voters.IsEmpty() {
				// If voters are exhausted and no decision was reached for the
				// (currently) highest priority candidate, decision cannot be
				// made for this frame with the provided DAG.
				break candidatesLoop
			}
			for voter := range voters.All() {
				yesCounter := consensus.NewVoteCounter(b.committee)
				noCounter := consensus.NewVoteCounter(b.committee)
				for prevFrameVoter, prevFrameVote := range prevFrameVotes {
					if b.dag.StronglyReaches(voter, prevFrameVoter) {
						if prevFrameVote {
							yesCounter.Vote(prevFrameVoter.Creator())
						} else {
							noCounter.Vote(prevFrameVoter.Creator())
						}
					}
				}
				// If the quorum is reached, make a definite decision for the candidate.
				if yesCounter.IsQuorumReached() {
					b.electedLeadersCache[frame] = candidate
					// electLeader will never be invoked again for a frame
					// lower or equal to lowestUndecidedFrame.
					b.lowestUndecidedFrame = frame + 1
					return candidate, ruledOutCandidates
				} else if noCounter.IsQuorumReached() {
					// This event is no longer the highest priority candidate,
					// stop the voting and election process for it completely.
					ruledOutCandidates = append(ruledOutCandidates, candidate)
					continue candidatesLoop
				}
				// If no quorum is reached, the voter places a vote based on simple majority.
				// Any of the two counters can be used to determine this vote.
				votes[voter] = yesCounter.GetVoteSum() >= noCounter.GetVoteSum()
			}
			prevFrameVotes = votes
		}
	}

	return nil, ruledOutCandidates
}

func (b *Moira) SortLeaders(events []*model.Event) []*model.Event {
	leaders := slices.DeleteFunc(events, func(event *model.Event) bool {
		return b.IsLeader(event) != layering.VerdictYes
	})

	slices.SortFunc(leaders, func(event1, event2 *model.Event) int {
		return b.getEventFrame(event1) - b.getEventFrame(event2)
	})

	return leaders
}

func (d *Moira) quorumOfRelations(source *model.Event, targets []*model.Event, relation func(source, target *model.Event) bool) bool {
	voteCounter := consensus.NewVoteCounter(d.committee)

	for _, base := range targets {
		if relation(source, base) {
			voteCounter.Vote(base.Creator())
		}
	}

	return voteCounter.IsQuorumReached()

}
