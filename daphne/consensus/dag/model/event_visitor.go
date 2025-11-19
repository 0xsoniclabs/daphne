package model

import "github.com/0xsoniclabs/daphne/daphne/consensus/dag/payload"

//go:generate mockgen -source event_visitor.go -destination=event_visitor_mock.go -package=model
//go:generate stringer -type=VisitResult -output event_visitor_string.go -trimprefix Visit_

// EventVisitor is an interface for visiting events during DAG traversal.
// It allows for custom logic and filtering to be executed on each event visited.
type EventVisitor[P payload.Payload] interface {
	// Visit should be called by the traversal algorithms on each event.
	// The Visit method returns a result that signals whether to continue,
	// prune the current branch, or abort the entire traversal.
	Visit(event *Event[P]) VisitResult
}

// WrapEventVisitor wraps a function with a signature func(event *Event) VisitResult
// into an EventVisitor adapter that can be used in traversal methods.
// This is a convenience function to allow using simple functions as event
// handlers without having to define a new type.
func WrapEventVisitor[P payload.Payload](f func(*Event[P]) VisitResult) EventVisitor[P] {
	return &eventVisitor[P]{visit: f}
}

type eventVisitor[P payload.Payload] struct {
	visit func(*Event[P]) VisitResult
}

func (v *eventVisitor[P]) Visit(event *Event[P]) VisitResult {
	return v.visit(event)
}

type VisitResult byte

const (
	// Visit_Descent indicates to continue descending in the current branch.
	Visit_Descent VisitResult = iota
	// Visit_Prune indicates to prune the current branch, continue with others.
	Visit_Prune
	// Visit_Abort indicates to abort the entire visit, the visitor has found what it needed.
	Visit_Abort
)
