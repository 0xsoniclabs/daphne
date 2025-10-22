package ruleset

import (
	"maps"
	"slices"
)

type Ruleset[T any] struct {
	rulesByPriority map[int][]*Rule[T]
}

func (rs *Ruleset[T]) AddRule(r *Rule[T], priority int) *Ruleset[T] {
	if rs.rulesByPriority == nil {
		rs.rulesByPriority = make(map[int][]*Rule[T])
	}
	rs.rulesByPriority[priority] = append(rs.rulesByPriority[priority], r)
	return rs
}

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

func (rs *Ruleset[T]) Reset() {
	for _, rules := range rs.rulesByPriority {
		for _, r := range rules {
			if r.onlyOnce {
				r.Reset()
			}
		}
	}
}

type Rule[T any] struct {
	conditions []Condition[T]
	action     Action[T]
	onlyOnce   bool
	executed   bool
}

func (r *Rule[T]) AddCondition(cond Condition[T]) *Rule[T] {
	r.conditions = append(r.conditions, cond)
	return r
}

func (r *Rule[T]) SetAction(act Action[T]) *Rule[T] {
	r.action = act
	return r
}

func (r *Rule[T]) Apply(val T) bool {
	if r.onlyOnce && r.executed {
		return false
	}
	for _, cond := range r.conditions {
		if !cond(val) {
			return false
		}
	}
	if r.action != nil {
		r.action(val)
	}
	r.executed = true
	return true
}

func (r *Rule[T]) OnlyOnce() *Rule[T] {
	r.onlyOnce = true
	return r
}

func (r *Rule[T]) Reset() *Rule[T] {
	r.executed = false
	return r
}

type Condition[T any] func(T) bool

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

func Not[T any](cond Condition[T]) Condition[T] {
	return func(val T) bool {
		return !cond(val)
	}
}

type Action[T any] func(T)
