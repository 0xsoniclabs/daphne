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

package model

//go:generate mockgen -source event_visitor.go -destination=event_visitor_mock.go -package=model
//go:generate stringer -type=VisitResult -output event_visitor_string.go -trimprefix Visit_

// EventVisitor is an interface for visiting events during DAG traversal.
// It allows for custom logic and filtering to be executed on each event visited.
type EventVisitor interface {
	// Visit should be called by the traversal algorithms on each event.
	// The Visit method returns a result that signals whether to continue,
	// prune the current branch, or abort the entire traversal.
	Visit(event *Event) VisitResult
}

// WrapEventVisitor wraps a function with a signature func(event *Event) VisitResult
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
