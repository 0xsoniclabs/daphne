// Copyright 2026 Sonic Labs
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

package moira

import (
	"cmp"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
	"github.com/0xsoniclabs/daphne/daphne/utils/sets"
)

// GenesisFrame is the frame number assigned to all genesis events. It is the
// lowest possible frame number in a Moira-layered DAG.
const GenesisFrame = 1

type Factory struct {
	CandidateLayerRelation eventRelationFunc
	VotingLayerRelation    eventRelationFunc
}

// eventRelationFunc defines a function type that determines whether a relation
// holds between two events.
type eventRelationFunc func(source, target *model.Event) bool

func (f Factory) String() string {
	// TODO: provide more detailed summary of the configuration based on the relations used.
	return "custom_moira"
}

// Moira layers the DAG by organizing events into frames and electing leaders
// among candidate events in each frame.
//
// Frame of an event is defined as one frame higher than the highest frame
// whose quorum of candidates forms a configurable relation (CandidateLayerRelation)
// with the event, having genesis events in frame 1 by definition.
//
// A fork is a pair of events created by the same creator where neither event
// can reach the other.
// DISCLAIMER: Fork detection is not implemented in the current version of the code.
// TODO(#399): implement fork detection
//
// Candidates are the first events in their frame by their creator.
//
// Leaders are elected from candidates based on a voting process involving a
// configurable relation called VotingLayerRelation, for initial voting, and the
// [model.Dag.StronglyReaches] relation, for aggregating and reaching consensus.
// The voters (events organized into Voting layers) cast and aggregate votes based
// on whether they have a VotingLayerRelation and whether they strongly reach a
// candidate, respectively.
//
// Voting layers are defined for each frame. The first voting layer (layer 0)
// consists of all events that strongly reach a quorum of candidates in the target frame,
// where there is no earlier event by the same creator that fulfills that condition.
// Subsequent voting layers are formed by events that strongly reach a quorum of
// voters from the previous layer. When a certain layer cannot reach a consensus,
// subsequent layers aggregate votes from the previous layer until consensus is reached.
//
// Moira allows for configuration of the relations used for candidate and immediate
// voting layer(layer 0), where the relation used for higher voting (aggregation/consensus)
// layers are fixed to [model.Dag.StronglyReaches].
// The Moira layering guarantees Byzantine Atomic Broadcast properties (agreement,
// integrity, validity, total order) when the relations used for candidate and voting
// layer relations are at least as "strong" as [model.Dag.Reaches].
type Moira struct {
	*Factory
	dag                  model.Dag
	committee            *consensus.Committee
	frameCache           map[model.EventId]int
	voterCache           map[voterSlot]map[*model.Event]bool
	electedLeadersCache  map[int]*model.Event
	lowestUndecidedFrame int
}

// voterSlot uniquely identifies a voter layer for a specific frame.
type voterSlot struct {
	frame int
	layer int
}

func (f Factory) NewLayering(dag model.Dag, committee *consensus.Committee) layering.Layering {
	return newMoira(&f, dag, committee)
}

func newMoira(config *Factory, dag model.Dag, committee *consensus.Committee) *Moira {
	// Default to using [model.Dag.Reaches] if any configurations are omitted.
	if config == nil {
		config = &Factory{
			CandidateLayerRelation: dag.Reaches,
			VotingLayerRelation:    dag.Reaches,
		}
	}
	if config.CandidateLayerRelation == nil {
		config.CandidateLayerRelation = dag.Reaches
	}
	if config.VotingLayerRelation == nil {
		config.VotingLayerRelation = dag.Reaches
	}
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

// IsCandidate returns true if the event is first in its frame by its creator.
func (m *Moira) IsCandidate(event *model.Event) bool {
	if event == nil || !slices.Contains(m.committee.Validators(), event.Creator()) {
		return false
	}
	// All genesis events are frame 1 candidates by definition.
	if event.IsGenesis() {
		return true
	}

	// From definition, a non-genesis event is a candidate if it has a different
	// frame than its self-parent.
	return m.getEventFrame(event.SelfParent()) != m.getEventFrame(event)
}

// IsLeader returns the Moira verdict for the given event.
// If the event is not a candidate, VerdictNo is returned.
// If the event is a candidate, the leader election process is executed
// for the candidate's frame - if a leader for the frame is elected and it
// is the provided candidate, VerdictYes is returned. If a different leader is
// elected, the verdict is No. If no leader can be elected for the candidate's
// frame with the provided DAG, VerdictUndecided is returned.
// The election process for a frame can only be executed if all previous frames
// have a decided leader, otherwise the verdict is VerdictUndecided.
func (m *Moira) IsLeader(candidate *model.Event) layering.Verdict {
	if !m.IsCandidate(candidate) {
		return layering.VerdictNo
	}

	// If at least one of the previous frames does not have a decided leader,
	// the decision for the candidate's frame cannot be made.
	for frame := m.lowestUndecidedFrame; frame < m.getEventFrame(candidate); frame++ {
		if event, _ := m.electLeader(frame); event == nil {
			return layering.VerdictUndecided
		}
	}

	switch event, eventsRuledOutAsLeaders := m.electLeader(m.getEventFrame(candidate)); {
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
func (m *Moira) SortLeaders(events []*model.Event) []*model.Event {
	leaders := slices.DeleteFunc(events, func(event *model.Event) bool {
		return m.IsLeader(event) != layering.VerdictYes
	})

	slices.SortFunc(leaders, func(event1, event2 *model.Event) int {
		return m.getEventFrame(event1) - m.getEventFrame(event2)
	})

	return leaders
}

// GetRound extracts the frame number for the event as its round.
func (m *Moira) GetRound(event *model.Event) uint32 {
	return uint32(m.getEventFrame(event))
}

// quorumOfRelations checks whether the source event has the specified relation
// with a quorum of the target events.
func (m *Moira) quorumOfRelations(source *model.Event, targets []*model.Event, relation eventRelationFunc) bool {
	voteCounter := consensus.NewVoteCounter(m.committee)

	for _, base := range targets {
		if relation(source, base) {
			voteCounter.Vote(base.Creator())
		}
	}

	return voteCounter.IsQuorumReached()

}

// electLeader attempts to elect a leader for the provided frame in the given DAG.
// If no leader can be elected with the provided DAG, nil is returned.
// It also returns all events that are decided with a NO verdict during the election process.
func (m *Moira) electLeader(frame int) (*model.Event, []*model.Event) {
	if electedLeader, alreadyElected := m.electedLeadersCache[frame]; alreadyElected {
		return electedLeader, nil
	}

	relevantEvents := sets.Empty[*model.Event]()

	// Collect all events relevant for the election in the target frame.
	// An event is relevant if it is a candidate in the target frame or
	// it is a potential (voter) (in higher or equal frame than the target one).
	for _, head := range m.dag.GetHeads() {
		head.TraverseClosure(model.WrapEventVisitor(
			func(e *model.Event) model.VisitResult {
				// Events that are in frames lower than the target frame, or are not candidates
				// (only candidates can be elected) are irrelevant for the election.
				if m.getEventFrame(e) < frame || relevantEvents.Contains(e) {
					return model.Visit_Prune
				}
				relevantEvents.Add(e)
				return model.Visit_Descent
			}),
		)
	}

	// Gather all the candidates in the target frame.
	candidates := sets.Filter(relevantEvents, func(e *model.Event) bool {
		return m.IsCandidate(e) && m.getEventFrame(e) == frame
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
		aStake := m.committee.GetValidatorStake(eventA.Creator())
		bStake := m.committee.GetValidatorStake(eventB.Creator())
		// Creator ID is used as a consistent tie-breaker.
		if aStake == bStake {
			return cmp.Compare(eventA.Creator(), eventB.Creator())
		}
		return cmp.Compare(bStake, aStake)

	})

	// Collect voters and group them into voting layers.
	// Terminate when no more voters are found for a layer.
	layeredVoters := map[int][]*model.Event{}
	for votingLayer := 0; ; votingLayer++ {
		// Layer 0 is built based on candidates in the target frame.
		// Higher layers are built based on voters in the previous layer.
		prevLayer := candidates
		if votingLayer != 0 {
			prevLayer = layeredVoters[votingLayer-1]
		}

		// Traverse relevant events ordered by their Seq numbers to capture
		// the earliest events which fulfill the voter condition, respectively by each creator.
		traverseOrder := slices.SortedFunc(relevantEvents.All(), func(a, b *model.Event) int {
			return cmp.Compare(a.Seq(), b.Seq())
		})

		voterSlot := voterSlot{
			frame: frame,
			layer: votingLayer,
		}
		if _, exists := m.voterCache[voterSlot]; !exists {
			m.voterCache[voterSlot] = make(map[*model.Event]bool)
		}

		// Keep track of validators for which the voter has been found so far.
		// Combined with the traversal order, this ensures that only the earliest
		// voter event per creator is selected.
		validatorsWithVoters := sets.Empty[consensus.ValidatorId]()
		voters := slices.DeleteFunc(traverseOrder, func(event *model.Event) bool {
			if isVoter, isCached := m.voterCache[voterSlot][event]; isCached {
				if isVoter {
					validatorsWithVoters.Add(event.Creator())
				}
				return !isVoter
			}
			if validatorsWithVoters.Contains(event.Creator()) {
				return true
			}
			isVoter := m.quorumOfRelations(event, prevLayer, m.dag.StronglyReaches)
			m.voterCache[voterSlot][event] = isVoter
			if isVoter {
				validatorsWithVoters.Add(event.Creator())
			}
			return !isVoter
		})

		if len(voters) == 0 {
			break
		}
		layeredVoters[votingLayer] = voters
		relevantEvents.Remove(voters...)
	}

	ruledOutCandidates := []*model.Event{}

candidatesLoop:
	for _, candidate := range candidates {
		// The election process for a single candidate consists of two types of rounds:
		// 1. Voting round: Collection of votes by voters from layer 0. The voter
		//   votes positively if it has a VotingLayerRelation with the candidate, and
		//   negatively otherwise.
		// 2. Aggregation(Consensus) round: Aggregation of votes from the previous round
		//  (previous layer voters). All rounds after the first voting round are aggregation
		//   rounds. If an aggregation round produces no decisions, the aggregating voters
		//   become the voters for the next round (next layer), by placing votes based on
		//   a simple majority. Layer 0 voters cannot aggregate nor reach consensus.

		// Round 1: collect the initial votes. A voter votes positively if it
		// has a VotingLayerRelation with the candidate, negatively otherwise.
		votes := map[*model.Event]bool{}
		for _, voter := range layeredVoters[0] {
			votes[voter] = m.VotingLayerRelation(voter, candidate)
		}
		// Aggregation rounds: If the aggregating voter strongly reaches a quorum
		// of voters from the previous layer, which voted the same for a specific
		// candidate, it makes a YES/NO decision for that candidate.
		prevFrameVotes := votes
		for votingLayer := 1; ; votingLayer++ {
			votes := map[*model.Event]bool{}
			voters := layeredVoters[votingLayer]
			if len(voters) == 0 {
				// If voters are exhausted and no decision was reached for the
				// (currently) highest priority candidate, decision cannot be
				// made for this frame with the provided DAG.
				break candidatesLoop
			}
			for _, voter := range voters {
				yesCounter := consensus.NewVoteCounter(m.committee)
				noCounter := consensus.NewVoteCounter(m.committee)
				for prevFrameVoter, prevFrameVote := range prevFrameVotes {
					if m.dag.StronglyReaches(voter, prevFrameVoter) {
						if prevFrameVote {
							yesCounter.Vote(prevFrameVoter.Creator())
						} else {
							noCounter.Vote(prevFrameVoter.Creator())
						}
					}
				}
				// If the quorum is reached, make a definite decision for the candidate.
				if yesCounter.IsQuorumReached() {
					m.electedLeadersCache[frame] = candidate
					// electLeader will never be invoked again for a frame
					// lower or equal to lowestUndecidedFrame.
					m.lowestUndecidedFrame = frame + 1
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

// getEventFrame computes the frame of an event and memoizes the result.
// The frame of an event is defined as one frame higher than the highest frame
// whose quorum of candidates has a CandidateLayerRelation with the target event.
// The equivalent definition would be that the frame of an event is the highest
// frame of its parents, plus one if and only if it has a CandidateLayerRelation
// with a quorum of candidates in that frame.
// All genesis events are by definition in frame 1.
func (m *Moira) getEventFrame(event *model.Event) int {
	if frame, ok := m.frameCache[event.EventId()]; ok {
		return frame
	}

	if event.IsGenesis() {
		m.frameCache[event.EventId()] = GenesisFrame
		return GenesisFrame
	}

	// Find the highest highestObservedFrame among parents.
	highestObservedFrame := GenesisFrame
	for _, parent := range event.Parents() {
		highestObservedFrame = max(highestObservedFrame, m.getEventFrame(parent))
	}

	highestObservedFrameCandidates := []*model.Event{}

	event.TraverseClosure(
		model.WrapEventVisitor(func(e *model.Event) model.VisitResult {
			if e == event {
				return model.Visit_Descent
			}
			if m.getEventFrame(e) < highestObservedFrame {
				return model.Visit_Prune
			}
			if m.getEventFrame(e) == highestObservedFrame && m.IsCandidate(e) {
				highestObservedFrameCandidates = append(highestObservedFrameCandidates, e)
			}
			return model.Visit_Descent
		}),
	)

	frame := highestObservedFrame
	if m.quorumOfRelations(event, highestObservedFrameCandidates, m.CandidateLayerRelation) {
		frame++
	}

	m.frameCache[event.EventId()] = frame
	return frame
}
