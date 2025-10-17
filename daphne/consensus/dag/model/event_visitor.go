package model

//go:generate mockgen -source event_visitor.go -destination=event_visitor_mock.go -package=model

// EventVisitor is an interface for visiting events during DAG traversal.
// It allows for custom logic and filtering to be executed on each event visited.
type EventVisitor interface {
	// Visit should be called by the traversal algorithms on each event.
	// If Visit returns true, it signals that further events on this branch
	// are of no interest to the visitor and thus can be skipped, i.e. the
	// branch can be pruned.
	Visit(event *Event) VisitResult
}

// WrapEventVisitor wraps a function with a signature func(event *Event) bool
// into an EventVisitor adapter that can be used in traversal methods.
// This is a convenience function to allow using simple functions as event
// handlers without having to define a new type.
func WrapEventVisitor(f func(*Event) VisitResult) EventVisitor {
	return &eventVisitor{visit: f}
}

type eventVisitor struct {
	visit func(*Event) VisitResult
}

func (v *eventVisitor) Visit(event *Event) VisitResult {
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
