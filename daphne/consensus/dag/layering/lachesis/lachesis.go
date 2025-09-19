package lachesis

import (
	"bytes"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/0xsoniclabs/daphne/daphne/consensus"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/layering"
	"github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"
)

const GenesisFrame = 1

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
	// All genesis events are considered frame 1 candidates.
	if event.IsGenesis() {
		return true
	}

	// Candidate is by definition an event that has a different frame than its self-parent.
	return l.eventFrame(event) != l.eventFrame(event.SelfParent())
}

func (l *Lachesis) IsLeader(dag *model.Dag, candidate *model.Event) layering.Verdict {
	if !l.IsCandidate(candidate) {
		return layering.VerdictNo
	}
	heads := dag.GetHeads()
	// Get all events that observe the candidate event
	voters := map[*model.Event]struct{}{}

	// Collect all events from the dag that reach the candidate event.
	for _, head := range heads {
		maps.Insert(voters, maps.All(head.GetClosure()))
	}

	// Competitors:
	competitors := maps.Clone(voters)
	maps.DeleteFunc(competitors, func(e *model.Event, _ struct{}) bool {
		return !l.IsCandidate(e) || l.eventFrame(e) != l.eventFrame(candidate)
	})
	competitors[candidate] = struct{}{}

	// Filter out events from the head's closure that:
	// 1) are not candidates themselves.
	// 2) have frame number less than or equal to the candidate's frame number.
	maps.DeleteFunc(voters, func(e *model.Event, _ struct{}) bool {
		return !l.IsCandidate(e) || l.eventFrame(e) <= l.eventFrame(candidate)
	})

	// Traverse candidates in a descending stake order.
	candidates := slices.Collect(maps.Keys(competitors))
	slices.SortFunc(candidates, func(a, b *model.Event) int {
		aStake, err := l.committee.GetCreatorStake(a.Creator())
		bStake, err2 := l.committee.GetCreatorStake(b.Creator())
		if err != nil || err2 != nil {
			slog.Warn("Failed to get creator stake", "err", err, "err2", err2)
			return 0
		}
		if aStake == bStake {
			return bytes.Compare(a.EventId().Serialize(), b.EventId().Serialize())
		}
		return int(aStake) - int(bStake)
	})
	for _, competitor := range candidates {
		prevFrameVotes := map[*model.Event]bool{}
		// Start voting from candidate event's frame + 1 and upwards.
		for frame := l.eventFrame(competitor) + 1; ; frame++ {
			frameVoters := maps.Clone(voters)
			maps.DeleteFunc(frameVoters, func(e *model.Event, _ struct{}) bool {
				return l.eventFrame(e) != frame
			})
			if len(frameVoters) == 0 {
				break
			}

			if frame == l.eventFrame(competitor)+1 {
				// In the first voting round, just collect the votes.
				for voter := range frameVoters {
					prevFrameVotes[voter] = l.stronglyReaches(voter, competitor)
				}
				continue
			} else {
				// If not the first voting round, do the aggregation.
				for voter := range frameVoters {
					// If a single voter from this frame strongy reaches a quorum of positive votes from
					// the previous frame, it decides the current
					yesCounter := consensus.NewVoteCounter(l.committee)
					noCounter := consensus.NewVoteCounter(l.committee)
					for prevFrameVoter, prevFrameVote := range prevFrameVotes {
						if prevFrameVote && l.stronglyReaches(voter, prevFrameVoter) {
							if err := yesCounter.Vote(prevFrameVoter.Creator()); err != nil {
								slog.Warn("Failed to register vote", "err", err)
							}
						} else {
							if err := noCounter.Vote(prevFrameVoter.Creator()); err != nil {
								slog.Warn("Failed to register vote", "err", err)
							}
						}
					}
					if yesCounter.IsQuorumReached() {
						if competitor == candidate {
							return layering.VerdictYes
						}
						return layering.VerdictNo
					} else if noCounter.IsQuorumReached() {
						if competitor == candidate {
							return layering.VerdictNo
						}
						// We can stop processing this competitor, as it has lost.
						break
					}
					prevFrameVotes[voter] = yesCounter.SimpleMajorityReached()
				}
			}
			// If we reach here, it means the current competitor is still in the race.
			// But a competitor with a higher priority is undecided and we have to
			// break the outer loop.
			break
		}
	}

	return layering.VerdictUndecided
}

func (l *Lachesis) eventFrame(event *model.Event) int {
	if frame, ok := l.frameCache[event.EventId()]; ok {
		fmt.Println("Frame of event", event.Creator(), event.Seq(), "is", frame)
		return frame
	}

	if event.IsGenesis() {
		l.frameCache[event.EventId()] = GenesisFrame
		fmt.Println("Frame of event", event.Creator(), event.Seq(), "is", GenesisFrame)
		return GenesisFrame
	}

	frame := GenesisFrame
	for _, parent := range event.Parents() {
		frame = max(frame, l.eventFrame(parent))
	}

	closure := event.GetClosure()
	delete(closure, event)

	// Find all ancestor events that are candidates.
	highestObservedFrameCandidates := slices.DeleteFunc(slices.Collect(maps.Keys(closure)), func(e *model.Event) bool {
		return l.eventFrame(e) != frame || !l.IsCandidate(e)
	})

	if l.stronglyReachesQuorum(event, highestObservedFrameCandidates) {
		frame++
	}

	l.frameCache[event.EventId()] = frame
	fmt.Println("Frame of event", event.Creator(), event.Seq(), "is", frame)
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

	transitEvents := source.GetClosure()

	// Get only the events from source's closure that observe the target
	maps.DeleteFunc(transitEvents, func(e *model.Event, _ struct{}) bool {
		_, seesTarget := e.GetClosure()[target]
		return !seesTarget
	})

	for e := range transitEvents {
		if err := voteCounter.Vote(e.Creator()); err != nil {
			slog.Warn("Failed to register vote", "err", err)
			return false
		}
	}

	return voteCounter.IsQuorumReached()
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
