// Copyright 2026 Sonic Operations Ltd
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

package ruleset

import (
	"maps"
	"slices"
)

// Ruleset represents a collection of rules that can be applied to values of type T.
// Rules are organized by priority, allowing for ordered evaluation.
type Ruleset[T any] struct {
	rulesByPriority map[int][]*Rule[T]
}

// AddRule adds a new Rule to the Ruleset with the specified priority. The lower it is,
// the earlier it will be evaluated. Within the same priority, there is no guaranteed order
// of evaluation.
func (rs *Ruleset[T]) AddRule(r *Rule[T], priority int) *Ruleset[T] {
	if rs.rulesByPriority == nil {
		rs.rulesByPriority = make(map[int][]*Rule[T])
	}
	rs.rulesByPriority[priority] = append(rs.rulesByPriority[priority], r)
	return rs
}

// Apply evaluates all rules in the ruleset against the provided value of type T.
// Rules are evaluated in order of their priority. Iff any rule is applied (i.e. its
// condition is met and its action is executed), the method returns true.
func (rs *Ruleset[T]) Apply(val T) bool {
	priorities := slices.Collect(maps.Keys(rs.rulesByPriority))
	slices.Sort(priorities)
	changed := false
	for _, priority := range priorities {
		for _, r := range rs.rulesByPriority[priority] {
			if r.Apply(val) {
				changed = true
			}
		}
	}
	return changed
}

// Reset resets all rules in the ruleset that are marked as "only once", allowing
// them to be applied again in future evaluations.
func (rs *Ruleset[T]) Reset() {
	for _, rules := range rs.rulesByPriority {
		for _, r := range rules {
			r.Reset()
		}
	}
}

// Rule represents a single rule with conditions and an action to be executed
// when the conditions are met. It can be configured to execute only once, or an
// unlimited number of times. Conditions and actions operate on values of type T.
type Rule[T any] struct {
	condition Condition[T]
	action    Action[T]
	onlyOnce  bool
	executed  bool
}

// SetCondition sets a new condition for the rule. The condition must be satisfied
// for the rule to be applied. Not setting a condition means the rule always applies.
func (r *Rule[T]) SetCondition(cond Condition[T]) *Rule[T] {
	r.condition = cond
	return r
}

// SetAction sets the action to be executed when the rule is applied.
// This is typically a mutation of the system.
func (r *Rule[T]) SetAction(act Action[T]) *Rule[T] {
	r.action = act
	return r
}

// Apply evaluates the rule against the provided value of type T.
// If the condition is met, the action is executed (if defined) and
// the method returns true. If the rule is marked as "only once" and
// has already been executed, it will not be applied again.
func (r *Rule[T]) Apply(val T) bool {
	if r.onlyOnce && r.executed {
		return false
	}
	if r.condition != nil && !r.condition(val) {
		return false
	}
	if r.action != nil {
		r.action(val)
	}
	r.executed = true
	return true
}

// OnlyOnce configures the rule to be executed only once. Subsequent attempts to apply
// the rule will have no effect until the rule is reset.
func (r *Rule[T]) OnlyOnce() *Rule[T] {
	r.onlyOnce = true
	return r
}

// Reset resets the rule's execution state, allowing it to be applied again
// if it is marked as "only once". Has no effect on rules that are not marked
// as "only once".
func (r *Rule[T]) Reset() *Rule[T] {
	r.executed = false
	return r
}

// Condition represents a predicate function that evaluates a value of type T
// and returns a boolean result. It is typically without side effects.
type Condition[T any] func(T) bool

// Or combines multiple conditions into a single condition that returns true
// if any of the provided conditions return true.
func Or[T any](conds ...Condition[T]) Condition[T] {
	return func(val T) bool {
		for _, cond := range conds {
			if cond(val) {
				return true
			}
		}
		return false
	}
}

// And combines multiple conditions into a single condition that returns true
// only if all of the provided conditions return true.
func And[T any](conds ...Condition[T]) Condition[T] {
	return func(val T) bool {
		for _, cond := range conds {
			if !cond(val) {
				return false
			}
		}
		return true
	}
}

// Not negates a condition, returning a new condition that yields the opposite result.
func Not[T any](cond Condition[T]) Condition[T] {
	return func(val T) bool {
		return !cond(val)
	}
}

// Action represents a function that performs an operation on a value of type T.
type Action[T any] func(T)
