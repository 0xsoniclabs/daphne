package sets

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet_Default_IsEmpty(t *testing.T) {
	var s Set[int]
	require.True(t, s.IsEmpty())
	require.Equal(t, 0, s.Size())
	require.Equal(t, Empty[int](), s)
	require.False(t, s.Contains(1))
	require.Nil(t, s.elements)
}

func TestSet_Default_CanBeUsed(t *testing.T) {
	var s Set[int]
	s.Add(1)
	require.True(t, s.Contains(1))
	require.Equal(t, 1, s.Size())
	s.Remove(1)
	require.True(t, s.IsEmpty())
	require.Equal(t, Empty[int](), s)
	require.Nil(t, s.elements)
}

func TestSet_Empty_ReturnsNilSet(t *testing.T) {
	empty := Empty[int]()
	require.Nil(t, empty.elements)
}

func TestSet_New_WithNoElements_ReturnsTheEmptySet(t *testing.T) {
	require.Equal(t, New[int](), Empty[int]())
}

func TestSet_New_WithElements_ContainsThoseElements(t *testing.T) {
	s := New(1, 2, 3)
	require.False(t, s.Contains(0))
	require.True(t, s.Contains(1))
	require.True(t, s.Contains(2))
	require.True(t, s.Contains(3))
	require.False(t, s.Contains(4))
}

func TestSet_Size_ReturnsNumberOfElements(t *testing.T) {
	s := Empty[int]()
	require.Equal(t, 0, s.Size())
	s.Add(1)
	require.Equal(t, 1, s.Size())
	s.Add(2, 3)
	require.Equal(t, 3, s.Size())
}

func TestSet_IsEmpty_ReturnsTrueForEmptySet(t *testing.T) {
	s := Empty[int]()
	require.True(t, s.IsEmpty())
	s.Add(1)
	require.False(t, s.IsEmpty())
}

func TestSet_Contains_ReturnsTrueForContainedElements(t *testing.T) {
	s := New(1, 2, 3)
	require.True(t, s.Contains(1))
	require.True(t, s.Contains(2))
	require.True(t, s.Contains(3))
}

func TestSet_Contains_ReturnsFalseForNonContainedElements(t *testing.T) {
	s := New(1, 2, 3)
	require.False(t, s.Contains(0))
	require.False(t, s.Contains(4))
}

func TestSet_IsSubsetOf_ReturnsTrueForSubset(t *testing.T) {
	sets := []Set[int]{
		Empty[int](),
		New(1),
		New(1, 2),
	}

	for i, s1 := range sets {
		for j, s2 := range sets {
			if i <= j {
				require.True(t, s1.IsSubsetOf(s2), "%v should be subset of %v", s1, s2)
			} else {
				require.False(t, s1.IsSubsetOf(s2), "%v should not be subset of %v", s1, s2)
			}
		}
	}
}

func TestSet_IsSubsetOf_ReturnsFalseForDifferentElements(t *testing.T) {
	s1 := New(1, 2)
	s2 := New(2, 3)
	require.False(t, s1.IsSubsetOf(s2))
	require.False(t, s2.IsSubsetOf(s1))
}

func TestSet_Equals_ReturnsTrueForEqualSets(t *testing.T) {
	sets := []Set[int]{
		Empty[int](),
		New(1),
		New(1, 2),
	}

	for i, s1 := range sets {
		for j, s2 := range sets {
			if i == j {
				require.True(t, s1.Equals(s2), "%v should equal %v", s1, s2)
			} else {
				require.False(t, s1.Equals(s2), "%v should not equal %v", s1, s2)
			}
		}
	}
}

func TestSet_All_IteratesOverAllElements(t *testing.T) {
	s := New(1, 2, 3)
	collected := []int{}
	for e := range s.All() {
		collected = append(collected, e)
	}
	require.ElementsMatch(t, []int{1, 2, 3}, collected)
}

func TestSet_All_IterationCanBeStoppedEarly(t *testing.T) {
	s := New(1, 2, 3)
	collected := []int{}
	for e := range s.All() {
		collected = append(collected, e)
		if len(collected) == 2 {
			break
		}
	}
	require.Len(t, collected, 2)
}

func TestSet_All_CanIterateEmptySet(t *testing.T) {
	s := Empty[int]()
	for e := range s.All() {
		t.Errorf("should not iterate over any elements, but got %v", e)
	}
}

func TestSet_Add_AddsElements(t *testing.T) {
	s := Empty[int]()
	s.Add(1)
	require.True(t, s.Contains(1))
	s.Add(2, 3)
	require.True(t, s.Contains(2))
	require.True(t, s.Contains(3))
}

func TestSet_Add_WithoutArguments_DoesNothing(t *testing.T) {
	e := Empty[int]()
	c := e.Clone()
	e.Add()
	require.Equal(t, c, e) // checks that a nil map is not replaced with an empty map
}

func TestSet_AddAll_AddsAllElementsFromOtherSet(t *testing.T) {
	s := New(1, 2)
	s.AddAll(New(2, 3))
	require.Equal(t, New(1, 2, 3), s)
}

func TestSet_AddAll_EmptyToEmpty_KeepsElementsNil(t *testing.T) {
	s := Empty[int]()
	s.AddAll(Empty[int]())
	require.Equal(t, Empty[int](), s)
	require.Nil(t, s.elements)
}

func TestSet_Remove_RemovesElements(t *testing.T) {
	s := New(1, 2, 3)
	s.Remove(2)
	require.False(t, s.Contains(2))
	require.True(t, s.Contains(1))
	require.True(t, s.Contains(3))
	s.Remove(1, 3)
	require.True(t, s.IsEmpty())
	require.Equal(t, Empty[int](), s)
}

func TestSet_Remove_WithoutArguments_DoesNothing(t *testing.T) {
	s := New(1, 2, 3)
	c := s.Clone()
	s.Remove()
	require.Equal(t, c, s)
}

func TestSet_RemoveAll_RemovesAllElementsFromOtherSet(t *testing.T) {
	s := New(1, 2, 3)
	s.RemoveAll(New(2, 3))
	require.Equal(t, New(1), s)
}

func TestSet_RemoveAll_EmptyFromEmpty_KeepsElementsNil(t *testing.T) {
	s := Empty[int]()
	s.RemoveAll(Empty[int]())
	require.Equal(t, Empty[int](), s)
	require.Nil(t, s.elements)
}

func TestSet_RemoveFunc_RejectEverything_DoesNothingToSet(t *testing.T) {
	none := func(int) bool { return false }
	s := New(1, 2, 3)
	s.RemoveFunc(none)
	require.Equal(t, New(1, 2, 3), s)

	s = Empty[int]()
	s.RemoveFunc(none)
	require.Equal(t, Empty[int](), s)
	require.Nil(t, s.elements)
}

func TestSet_RemoveFunc_AcceptEverything_ProducesAnEmptySet(t *testing.T) {
	all := func(int) bool { return true }
	s := New(1, 2, 3)
	s.RemoveFunc(all)
	require.Equal(t, Empty[int](), s)
	require.Nil(t, s.elements)

	s = Empty[int]()
	s.RemoveFunc(all)
	require.Equal(t, Empty[int](), s)
	require.Nil(t, s.elements)
}

func TestSet_RemoveFunc_SelectivePredicate_RetainsNonMatchingElements(t *testing.T) {
	odd := func(i int) bool { return i%2 != 0 }
	s := New(1, 2, 3, 4, 5)
	s.RemoveFunc(odd)
	require.Equal(t, New(2, 4), s)
}

func TestSet_Clone_CreatesIndependentCopy(t *testing.T) {
	s := New(1, 2, 3)
	c := s.Clone()
	require.Equal(t, s, c)
	s.Add(4)
	require.NotEqual(t, s, c)
	c.Remove(1)
	require.NotEqual(t, s, c)
}

func TestSet_ToSlice_ReturnsAllElementsAsSlice(t *testing.T) {
	s := New(1, 2, 3)
	slice := s.ToSlice()
	require.ElementsMatch(t, []int{1, 2, 3}, slice)
}

func TestSet_ToSlice_EmptySet_ReturnsNilSlice(t *testing.T) {
	s := Empty[int]()
	slice := s.ToSlice()
	require.Empty(t, slice)
	require.Nil(t, slice)
}

func TestSet_String_EmptySet_IsBraces(t *testing.T) {
	s := Empty[int]()
	require.Equal(t, "{}", s.String())
}

func TestSet_String_SortsElements(t *testing.T) {
	s := New(3, 1, 2)
	require.Equal(t, "{1, 2, 3}", s.String())
}

func TestUnion_NoArgument_ProducesEmptySet(t *testing.T) {
	require.Equal(t, Empty[int](), Union[int]())
}

func TestUnion_SingleArgument_ProducesCloneOfArgument(t *testing.T) {
	s := New(1, 2, 3)
	r := Union(s)
	require.Equal(t, s, r)
	s.Add(4)
	require.NotEqual(t, s, r)
}

func TestUnion_MultipleArguments_ProducesUnionOfAll(t *testing.T) {
	s1 := New(1, 2)
	s2 := New(2, 3)
	s3 := New(3, 4)
	r := Union(s1, s2, s3)
	require.Equal(t, New(1, 2, 3, 4), r)
}

func TestIntersection_NoArgument_ProducesEmptySet(t *testing.T) {
	require.Equal(t, Empty[int](), Intersection[int]())
}

func TestIntersection_SingleArgument_ProducesCloneOfArgument(t *testing.T) {
	s := New(1, 2, 3)
	r := Intersection(s)
	require.Equal(t, s, r)
	s.Add(4)
	require.NotEqual(t, s, r)
}

func TestIntersection_MultipleArguments_ProducesIntersectionOfAll(t *testing.T) {
	s1 := New(1, 2, 3)
	s2 := New(2, 3, 4)
	s3 := New(3, 4, 5)
	r := Intersection(s1, s2, s3)
	require.Equal(t, New(3), r)
}

func TestIntersection_NoCommonElements_ProducesEmptySet(t *testing.T) {
	s1 := New(1, 2)
	s2 := New(3, 4)
	r := Intersection(s1, s2)
	require.Equal(t, Empty[int](), r)
}

func TestIntersection_OneEmptySet_ProducesEmptySet(t *testing.T) {
	s1 := New(1, 2)
	s2 := Empty[int]()
	r := Intersection(s1, s2)
	require.Equal(t, Empty[int](), r)
}

func TestIntersection_AllEmptySets_ProducesEmptySet(t *testing.T) {
	s1 := Empty[int]()
	s2 := Empty[int]()
	r := Intersection(s1, s2)
	require.Equal(t, Empty[int](), r)
}

func TestDifference_EmptyStartSet_ProducesEmptySet(t *testing.T) {
	s1 := Empty[int]()
	s2 := New(1, 2)
	r := Difference(s1, s2)
	require.Equal(t, Empty[int](), r)
}

func TestDifference_EmptySubtractSet_ProducesCloneOfStartSet(t *testing.T) {
	s1 := New(1, 2, 3)
	s2 := Empty[int]()
	r := Difference(s1, s2)
	require.Equal(t, s1, r)
	s1.Add(4)
	require.NotEqual(t, s1, r)
}

func TestDifference_NonEmptySets_ProducesDifference(t *testing.T) {
	s1 := New(1, 2, 3)
	s2 := New(2, 4)
	r := Difference(s1, s2)
	require.Equal(t, New(1, 3), r)
}

func TestDifference_NoCommonElements_ProducesCloneOfStartSet(t *testing.T) {
	s1 := New(1, 2, 3)
	s2 := New(4, 5)
	r := Difference(s1, s2)
	require.Equal(t, s1, r)
	s1.Add(4)
	require.NotEqual(t, s1, r)
}

func TestMap_EmptySet_ProducesEmptySet(t *testing.T) {
	s := Empty[int]()
	mapped := Map(s, func(i int) string {
		return string(rune('a' + i))
	})
	require.Equal(t, Empty[string](), mapped)
}

func TestMap_NonEmptySet_ProducesMappedSet(t *testing.T) {
	s := New(0, 1, 2)
	mapped := Map(s, func(i int) string {
		return string(rune('a' + i))
	})
	require.Equal(t, New("a", "b", "c"), mapped)
}

func TestFilter_EmptySet_ProducesEmptySet(t *testing.T) {
	s := Empty[int]()
	filtered := Filter(s, func(i int) bool {
		return i%2 == 0
	})
	require.Equal(t, Empty[int](), filtered)
}

func TestFilter_NonEmptySet_ProducesFilteredSet(t *testing.T) {
	s := New(1, 2, 3, 4, 5)
	filtered := Filter(s, func(i int) bool {
		return i%2 == 0
	})
	require.Equal(t, New(2, 4), filtered)
}

func TestFilter_NoneMatch_ProducesEmptySet(t *testing.T) {
	s := New(1, 2, 3)
	filtered := Filter(s, func(i int) bool {
		return i > 10
	})
	require.Equal(t, Empty[int](), filtered)
}

func TestFilter_AllMatch_ProducesSameSet(t *testing.T) {
	s := New(1, 2, 3)
	filtered := Filter(s, func(i int) bool {
		return i > 0
	})
	require.Equal(t, s, filtered)
}

func TestReduce_EmptySet_ReturnsInitialValue(t *testing.T) {
	s := Empty[int]()
	sum := Reduce(s, func(acc, e int) int {
		return acc + e
	}, 10)
	require.Equal(t, 10, sum)
}

func TestReduce_NonEmptySet_ReturnsReducedValue(t *testing.T) {
	s := New(1, 2, 3)
	sum := Reduce(s, func(acc, e int) int {
		return acc + e
	}, 0)
	require.Equal(t, 6, sum)
}

func TestAny_EmptySet_ReturnsFalse(t *testing.T) {
	s := Empty[int]()
	result := Any(s, func(e int) bool {
		return true // Always true, but set is empty.
	})
	require.False(t, result)
}

func TestAny_NonEmptySet_ElementMatchesPredicate_ReturnsTrue(t *testing.T) {
	s := New(1, 2, 3)
	result := Any(s, func(e int) bool {
		return e == 2
	})
	require.True(t, result)
}

func TestAny_NonEmptySet_NoElementMatchesPredicate_ReturnsFalse(t *testing.T) {
	s := New(1, 2, 3)
	result := Any(s, func(e int) bool {
		return e > 10
	})
	require.False(t, result)
}

func TestAll_EmptySet_ReturnsTrue(t *testing.T) {
	s := Empty[int]()
	result := All(s, func(e int) bool {
		return false // Always false, but set is empty.
	})
	require.True(t, result)
}

func TestAll_NonEmptySet_AllElementsMatchPredicate_ReturnsTrue(t *testing.T) {
	s := New(2, 4, 6)
	result := All(s, func(e int) bool {
		return e%2 == 0
	})
	require.True(t, result)
}

func TestAll_NonEmptySet_AtLeastOneElementDoesNotMatchPredicate_ReturnsFalse(t *testing.T) {
	s := New(1, 2, 3)
	result := All(s, func(e int) bool {
		return e%2 == 0
	})
	require.False(t, result)
}
