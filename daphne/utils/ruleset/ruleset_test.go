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

package ruleset

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCondition_Not_Negates(t *testing.T) {
	trueCond := func(x int) bool { return x > 10 }
	falseCond := Not(trueCond)
	require.True(t, falseCond(5))
	require.False(t, falseCond(15))
}

func TestCondition_Or_DoesLogicalOr(t *testing.T) {
	cond1 := func(x int) bool { return x > 10 }
	cond2 := func(x int) bool { return x%2 == 0 }
	orCond := Or(cond1, cond2)

	require.True(t, orCond(12))
	require.True(t, orCond(8))
	require.False(t, orCond(7))
}

func TestCondition_And_DoesLogicalAnd(t *testing.T) {
	cond1 := func(x int) bool { return x > 10 }
	cond2 := func(x int) bool { return x%2 == 0 }
	andCond := And(cond1, cond2)

	require.True(t, andCond(12))
	require.False(t, andCond(8))
	require.False(t, andCond(7))
}

func TestRule_NilConditionTriviallyTrue(t *testing.T) {
	rule := &Rule[int]{}
	require.True(t, rule.Apply(5), "Nil condition should trivially return true")
}

func TestRule_Apply_ExecutesActionWhenConditionsMet(t *testing.T) {
	rule := &Rule[int]{}
	cond1 := func(x int) bool { return x > 10 }
	cond2 := func(x int) bool { return x%2 == 0 }
	actionExecuted := false
	action := func(x int) {
		actionExecuted = true
	}

	rule.SetCondition(And(cond1, cond2))
	rule.SetAction(action)

	ret := rule.Apply(3)
	require.False(t, actionExecuted, "Action should not execute when conditions are not met")
	require.False(t, ret, "Apply should return false when conditions are not met")

	ret = rule.Apply(12)
	require.True(t, actionExecuted, "Action should execute when conditions are met")
	require.True(t, ret, "Apply should return true when conditions are met")
}

func TestRule_Apply_NothingHappensWhenActionIsNil(t *testing.T) {
	rule := &Rule[int]{}
	cond := func(x int) bool { return x > 0 }

	rule.SetCondition(cond)

	ret := rule.Apply(5)
	require.True(t, ret,
		"Apply should return true when conditions are met even if action is nil")
}

func TestRule_OnlyOnce_PreventsMultipleExecutions(t *testing.T) {
	rule := &Rule[int]{}
	cond := func(x int) bool { return x > 0 }
	executionCount := 0
	action := func(x int) {
		executionCount++
	}

	rule.SetCondition(cond)
	rule.SetAction(action)
	rule.OnlyOnce()

	require.True(t, rule.Apply(5), "First Apply should return true")
	require.False(t, rule.Apply(10), "Second Apply should return false")
	require.Equal(t, 1, executionCount, "Action should execute only once")
}

func TestRule_Reset_AllowsReexecution(t *testing.T) {
	rule := &Rule[int]{}
	cond := func(x int) bool { return x > 0 }
	executionCount := 0
	action := func(x int) {
		executionCount++
	}

	rule.SetCondition(cond)
	rule.SetAction(action)
	rule.OnlyOnce()

	require.True(t, rule.Apply(5), "First Apply should return true")
	rule.Reset()
	require.True(t, rule.Apply(10), "Apply after Reset should return true")
	require.Equal(t, 2, executionCount, "Action should execute twice after Reset")
}

func TestRuleset_AddRule_AddsRulesToCorrectPriority(t *testing.T) {
	rs := Ruleset[int]{}
	rule1 := &Rule[int]{}
	rule2 := &Rule[int]{}

	rs.AddRule(rule1, 1)
	rs.AddRule(rule2, 2)
	require.Equal(t, 1, len(rs.rulesByPriority[1]), "Priority 1 should have 1 rule")
	require.Equal(t, 1, len(rs.rulesByPriority[2]), "Priority 2 should have 1 rule")
	require.Equal(t, 2, len(rs.rulesByPriority), "Ruleset should have 2 priorities")
}

func TestRuleset_Apply_ReportsIfAnyRuleApplied(t *testing.T) {
	rs := Ruleset[int]{}
	rule1 := &Rule[int]{}
	rule1.SetCondition(func(x int) bool { return x > 10 })
	rule2 := &Rule[int]{}
	rule2.SetCondition(func(x int) bool { return x < 0 })

	rs.AddRule(rule1, 1)
	rs.AddRule(rule2, 2)

	changed := rs.Apply(0)
	require.False(t, changed, "No rules should apply for input 0")

	changed = rs.Apply(15)
	require.True(t, changed, "At least one rule should apply for input 15")
}

func TestRuleset_Apply_AppliesRulesInPriorityOrder(t *testing.T) {
	rs := Ruleset[int]{}
	executionOrder := []int{}

	auxFunc := func(x int) func(_ int) {
		return func(_ int) {
			executionOrder = append(executionOrder, x)
		}
	}

	rule1 := &Rule[int]{}
	rule1.SetAction(auxFunc(1))
	rule2 := &Rule[int]{}
	rule2.SetAction(auxFunc(2))
	rule3 := &Rule[int]{}
	rule3.SetAction(auxFunc(3))

	rs.AddRule(rule1, 1)
	rs.AddRule(rule2, 2)
	rs.AddRule(rule3, 3)
	rs.Apply(0)

	require.Equal(t, []int{1, 2, 3}, executionOrder, "Rules should execute in priority order")
}

func TestRuleset_Reset_ResetsOnlyOnceRules(t *testing.T) {
	rs := Ruleset[int]{}
	executionCount := 0

	rule := &Rule[int]{}
	rule.SetAction(func(_ int) {
		executionCount++
	})
	rule.OnlyOnce()

	rs.AddRule(rule, 1)

	require.True(t, rs.Apply(5), "First Apply should return true")
	require.Equal(t, 1, executionCount, "Action should execute once")
	require.False(t, rs.Apply(10), "Second Apply should return false")
	require.Equal(t, 1, executionCount, "Action should not execute again")

	rs.Reset()

	require.True(t, rs.Apply(15), "Apply after Reset should return true")
	require.Equal(t, 2, executionCount, "Action should execute again after Reset")
}
