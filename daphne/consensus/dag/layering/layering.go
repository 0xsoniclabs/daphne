package layering

import "github.com/0xsoniclabs/daphne/daphne/consensus/dag/model"

//go:generate stringer -type=Verdict -output layering_string.go -trimprefix Verdict

type Verdict int

const (
	VerdictYes Verdict = iota
	VerdictNo
	VerdictUndecided
)

type Layering interface {
	IsCandidate(event model.Event) (bool, error)
	IsVoter(event model.Event) (bool, error)
	IsLeader(event model.Event) (Verdict, error)
	SortLeaders(events []model.Event) ([]model.Event, error)
}
