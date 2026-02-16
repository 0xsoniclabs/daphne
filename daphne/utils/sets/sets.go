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

// The set package provides a generic set data structure for elements of any
// comparable type in Go. It supports standard set operations such as addition,
// removal, union, intersection, and difference. The set is implemented using a
// map for efficient membership testing and manipulation.
//
// To create a new set, use either the Empty or New functions.
//
//	s := sets.Empty[int]()        // creates an empty set of integers
//	s := sets.New(1, 2, 3)        // creates a set with elements 1, 2, and 3
//
// Among others, the following operations are supported on sets:
//   - Add, AddAll: to add elements to the set
//   - Remove, RemoveAll, RemoveFunc: to remove elements from the set
//   - Contains: to check if an element is in the set
//   - IsSubsetOf, Equals: to compare sets
//
// To iterate over all elements in the set, use the All method which returns an
// iterator function:
//
//	s := sets.New(1, 2, 3)
//	for e := range s.All() {
//	    fmt.Println(e) // prints 1, 2, and 3 in no particular order
//	}
//
// Furthermore, the following stand-alone functions are provided:
//   - Union        .. combines multiple sets into one set containing all elements
//   - Intersection .. finds common elements across multiple sets
//   - Difference   .. finds elements present in one set but not in the other
//   - Map          .. transforms elements of a set using a provided function
//   - Filter       .. filters elements of a set based on a predicate function
//   - Reduce       .. reduces the set to a single value using an accumulator function
//   - Any          .. checks if any element in the set satisfies a predicate
//   - All          .. checks if all elements in the set satisfy a predicate
//
// These free-standing functions do not alter the original sets; instead, they
// return a new set or value as appropriate.
//
// The provided Set type uses a map[T]struct{} internally to store elements. Set
// operations exhibit the corresponding runtime complexities.
package sets

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"
	"sort"
	"strings"
)

// Set is a generic set data structure for elements of any comparable type T.
// It supports standard set operations such as addition, removal, union,
// intersection, and difference. The set is implemented using a map for
// efficient membership testing and manipulation.
type Set[T comparable] struct {
	elements map[T]struct{}
}

// Empty returns a new empty set.
func Empty[T comparable]() Set[T] {
	return New[T]()
}

// New creates a new set containing the provided elements. If no elements are
// provided, it returns an empty set.
func New[T comparable](elements ...T) Set[T] {
	if len(elements) == 0 {
		return Set[T]{}
	}
	s := Set[T]{}
	s.Add(elements...)
	return s
}

// FromIter creates a new set with the elements from the provided iterator.
func FromIter[T comparable](it iter.Seq[T]) Set[T] {
	s := Empty[T]()
	for e := range it {
		s.Add(e)
	}
	return s
}

// Size returns the number of elements in the set.
// This operation runs in O(1) time.
func (s *Set[T]) Size() int {
	return len(s.elements)
}

// IsEmpty returns true if the set contains no elements.
// This operation runs in O(1) time.
func (s *Set[T]) IsEmpty() bool {
	return s.Size() == 0
}

// Contains returns true if the set contains the specified element.
// This operation runs in O(1) time.
func (s Set[T]) Contains(e T) bool {
	_, ok := s.elements[e]
	return ok
}

// IsSubsetOf returns true if the set is a subset of the other set or equal (⊆).
// This operation runs in O(n) time, where n is the size of this set.
func (s Set[T]) IsSubsetOf(other Set[T]) bool {
	if s.Size() > other.Size() {
		return false
	}
	for e := range s.All() {
		if !other.Contains(e) {
			return false
		}
	}
	return true
}

// Equals returns true if the set is equal to the other set.
// This operation runs in O(n) time, where n is the size of this set.
func (s Set[T]) Equals(other Set[T]) bool {
	return s.IsSubsetOf(other) && other.IsSubsetOf(s)
}

// All returns an iterator over all elements in the set. The order of elements
// is not guaranteed.
func (s Set[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for e := range s.elements {
			if !yield(e) {
				return
			}
		}
	}
}

// Add adds zero or more elements to the set. If an element is already present,
// it is not added again.
// This operation runs in O(m) time, where m is the number of elements to add.
func (s *Set[T]) Add(e ...T) {
	if s.elements == nil && len(e) > 0 {
		s.elements = make(map[T]struct{})
	}
	for _, v := range e {
		s.elements[v] = struct{}{}
	}
}

// AddAll adds all elements from the other set to this set.
// This operation runs in O(n) time, where n is the size of the other set.
func (s *Set[T]) AddAll(other Set[T]) {
	for e := range other.elements {
		s.Add(e)
	}
}

// Remove removes zero or more elements from the set. If an element is not
// present, it is ignored.
// This operation runs in O(m) time, where m is the number of elements to remove.
func (s *Set[T]) Remove(e ...T) {
	for _, v := range e {
		delete(s.elements, v)
	}
	if len(s.elements) == 0 {
		s.elements = nil
	}
}

// RemoveAll removes all elements in the other set from this set.
// This operation runs in O(n) time, where n is the size of the other set.
func (s *Set[T]) RemoveAll(other Set[T]) {
	s.Remove(slices.Collect(other.All())...)
}

// RemoveFunc removes all elements from the set that satisfy the given
// predicate function. The predicate is called for each element in the set.
// This operation runs in O(n) time, where n is the size of the set.
func (s *Set[T]) RemoveFunc(predicate func(T) bool) {
	var toDelete []T
	for e := range s.All() {
		if predicate(e) {
			toDelete = append(toDelete, e)
		}
	}
	s.Remove(toDelete...)
}

// Clone returns a shallow copy of the set.
func (s Set[T]) Clone() Set[T] {
	return Set[T]{elements: maps.Clone(s.elements)}
}

// ToSlice returns a slice containing all elements in the set. The order of
// elements is not guaranteed.
func (s Set[T]) ToSlice() []T {
	return slices.Collect(maps.Keys(s.elements))
}

// String returns a string representation of the set in the form "{e1, e2, ...}".
// The elements are sorted lexicographically for consistent output.
func (s Set[T]) String() string {
	elements := s.ToSlice()
	parts := make([]string, len(elements))
	for i, e := range elements {
		parts[i] = fmt.Sprintf("%v", e)
	}
	sort.Strings(parts)
	return "{" + strings.Join(parts, ", ") + "}"
}

// Union returns a new set that is the union of all provided sets. If no sets
// are provided, it returns an empty set. If a single set is provided, it
// returns a clone of that set.
func Union[T comparable](a ...Set[T]) Set[T] {
	result := New[T]()
	for _, set := range a {
		result.AddAll(set)
	}
	return result
}

// Intersection returns a new set that is the intersection of all provided sets.
// If no sets are provided, it returns an empty set. If a single set is
// provided, it returns a clone of that set.
func Intersection[T comparable](a ...Set[T]) Set[T] {
	if len(a) == 0 {
		return New[T]()
	}
	if len(a) == 1 {
		return a[0].Clone()
	}
	result := New[T]()
	slices.SortFunc(a, func(s1, s2 Set[T]) int {
		return cmp.Compare(s1.Size(), s2.Size())
	})
	for e := range a[0].All() {
		inAll := true
		for _, set := range a[1:] {
			if !set.Contains(e) {
				inAll = false
				break
			}
		}
		if inAll {
			result.Add(e)
		}
	}
	return result
}

// Difference returns a new set that contains elements in set 'a' that are not
// in set 'b'. If 'a' is empty, it returns an empty set. If 'b' is empty, it
// returns a clone of 'a'.
func Difference[T comparable](a, b Set[T]) Set[T] {
	res := New[T]()
	for e := range a.All() {
		if !b.Contains(e) {
			res.Add(e)
		}
	}
	return res
}

// Map returns a new set whose elements are the result of applying the function f
// to each element of the input set s. The resulting set contains elements of type U.
func Map[T comparable, U comparable](s Set[T], f func(T) U) Set[U] {
	res := New[U]()
	for e := range s.All() {
		res.Add(f(e))
	}
	return res
}

// Filter returns a new set containing only the elements of the input set s that
// satisfy the predicate function f.
func Filter[T comparable](s Set[T], f func(T) bool) Set[T] {
	res := s.Clone()
	res.RemoveFunc(func(e T) bool {
		return !f(e)
	})
	return res
}

// Reduce applies the function f to each element of the set s, accumulating a single
// result of type U. The initial value for the accumulation is provided as 'initial'.
// Note that the order of element processing is not guaranteed and may affect the result,
// thus f should be associative and commutative for consistent results.
func Reduce[T comparable, U any](s Set[T], f func(U, T) U, initial U) U {
	result := initial
	for e := range s.All() {
		result = f(result, e)
	}
	return result
}

// Any returns true if any element in the set satisfies the predicate function f.
func Any[T comparable](s Set[T], f func(T) bool) bool {
	for e := range s.All() {
		if f(e) {
			return true
		}
	}
	return false
}

// All returns true if all elements in the set satisfy the predicate function f.
func All[T comparable](s Set[T], f func(T) bool) bool {
	return !Any(s, func(e T) bool { return !f(e) })
}
