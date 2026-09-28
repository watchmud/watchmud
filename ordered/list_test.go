package ordered

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// thing is a stand-in for the instances this list actually holds: identified
// by an id, found by a name the player typed.
type thing struct {
	id   int
	name string
}

func (t *thing) Id() int { return t.id }

func (t *thing) Matches(target string) bool {
	return strings.EqualFold(t.name, target)
}

func newTestList() *List[int, *thing] {
	return NewList[int]((*thing).Id)
}

func collect(l *List[int, *thing]) []*thing {
	return slices.Collect(l.All())
}

func TestAddKeepsInsertionOrder(t *testing.T) {
	l := newTestList()
	one, two, three := &thing{1, "lizard"}, &thing{2, "lizard"}, &thing{3, "rat"}

	require.NoError(t, l.Add(one))
	require.NoError(t, l.Add(two))
	require.NoError(t, l.Add(three))

	assert.Equal(t, []*thing{one, two, three}, collect(l))
	assert.Equal(t, 3, l.Len())
}

// one added, two added, one leaves, three added -> [two, three]
func TestRemoveFromMiddleKeepsOrder(t *testing.T) {
	l := newTestList()
	one, two, three := &thing{1, "lizard"}, &thing{2, "lizard"}, &thing{3, "rat"}

	require.NoError(t, l.Add(one))
	require.NoError(t, l.Add(two))
	require.NoError(t, l.Remove(one))
	require.NoError(t, l.Add(three))

	assert.Equal(t, []*thing{two, three}, collect(l))
}

func TestAddDuplicateKeyIsErrDuplicate(t *testing.T) {
	l := newTestList()
	require.NoError(t, l.Add(&thing{1, "lizard"}))

	// a different value carrying a key that is already taken
	err := l.Add(&thing{1, "rat"})

	assert.True(t, errors.Is(err, ErrDuplicate))
	assert.Equal(t, 1, l.Len())
	assert.Equal(t, "lizard", collect(l)[0].name)
}

func TestRemoveMissingIsErrNotFound(t *testing.T) {
	l := newTestList()

	assert.True(t, errors.Is(l.Remove(&thing{1, "lizard"}), ErrNotFound))
	assert.True(t, errors.Is(l.RemoveKey(1), ErrNotFound))
	assert.Empty(t, collect(l))
}

// Remove finds the entry by key, not by pointer identity, so removing a stale
// copy of a value still empties the slot.
func TestRemoveByKeyNotIdentity(t *testing.T) {
	l := newTestList()
	require.NoError(t, l.Add(&thing{1, "lizard"}))

	require.NoError(t, l.Remove(&thing{1, "a different lizard struct"}))

	assert.Empty(t, collect(l))
	assert.False(t, l.Contains(1))
}

func TestRemoveEverything(t *testing.T) {
	l := newTestList()
	one, two := &thing{1, "lizard"}, &thing{2, "rat"}
	require.NoError(t, l.Add(one))
	require.NoError(t, l.Add(two))

	require.NoError(t, l.Remove(two))
	require.NoError(t, l.Remove(one))

	assert.Empty(t, collect(l))
	assert.Equal(t, 0, l.Len())
}

// The removed value must not stay reachable through the backing array, which
// is the bug append(order[:i], order[i+1:]...) leaves behind.
func TestRemoveClearsTheTail(t *testing.T) {
	l := newTestList()
	one, two := &thing{1, "lizard"}, &thing{2, "rat"}
	require.NoError(t, l.Add(one))
	require.NoError(t, l.Add(two))

	require.NoError(t, l.Remove(one))

	tail := l.order[:cap(l.order)][1]
	assert.Nil(t, tail, "removed value is still referenced past len")
}

func TestGet(t *testing.T) {
	l := newTestList()
	one := &thing{1, "lizard"}
	require.NoError(t, l.Add(one))

	got, exists := l.Get(1)
	assert.True(t, exists)
	assert.Same(t, one, got)

	got, exists = l.Get(99)
	assert.False(t, exists)
	assert.Nil(t, got)
}

func TestAllExcept(t *testing.T) {
	l := newTestList()
	one, two, three := &thing{1, "lizard"}, &thing{2, "rat"}, &thing{3, "bat"}
	require.NoError(t, l.Add(one))
	require.NoError(t, l.Add(two))
	require.NoError(t, l.Add(three))

	assert.Equal(t, []*thing{one, three}, slices.Collect(l.AllExcept(2)))
	assert.Equal(t, []*thing{one, two, three}, slices.Collect(l.AllExcept(99)))
}

// break out of a range over All / AllExcept
func TestIteratorsStopEarly(t *testing.T) {
	l := newTestList()
	require.NoError(t, l.Add(&thing{1, "lizard"}))
	require.NoError(t, l.Add(&thing{2, "rat"}))
	require.NoError(t, l.Add(&thing{3, "bat"}))

	var seen []string
	for v := range l.AllExcept(1) {
		seen = append(seen, v.name)
		break
	}
	assert.Equal(t, []string{"rat"}, seen)
}

func TestSliceIsACopy(t *testing.T) {
	l := newTestList()
	one, two := &thing{1, "lizard"}, &thing{2, "rat"}
	require.NoError(t, l.Add(one))
	require.NoError(t, l.Add(two))

	held := l.Slice()
	require.NoError(t, l.Remove(one))

	assert.Equal(t, []*thing{one, two}, held)
	assert.Equal(t, []*thing{two}, collect(l))
}

// Two identical lizards: the answer is the one that got there first, and it
// stays the same answer until that one leaves.
func TestFindReturnsFirstAdded(t *testing.T) {
	l := newTestList()
	one, two := &thing{1, "lizard"}, &thing{2, "lizard"}
	require.NoError(t, l.Add(one))
	require.NoError(t, l.Add(two))

	found, exists := Find(l, "lizard")
	require.True(t, exists)
	assert.Same(t, one, found)

	require.NoError(t, l.Remove(one))
	found, exists = Find(l, "lizard")
	require.True(t, exists)
	assert.Same(t, two, found)
}

func TestFindNotFound(t *testing.T) {
	l := newTestList()
	require.NoError(t, l.Add(&thing{1, "lizard"}))

	found, exists := Find(l, "goblin")
	assert.False(t, exists)
	assert.Nil(t, found, "the zero value of T, not a leftover")
}

func TestFindAll(t *testing.T) {
	l := newTestList()
	one, two, three := &thing{1, "lizard"}, &thing{2, "rat"}, &thing{3, "lizard"}
	require.NoError(t, l.Add(one))
	require.NoError(t, l.Add(two))
	require.NoError(t, l.Add(three))

	assert.Equal(t, []*thing{one, three}, FindAll(l, "lizard"))
	assert.Empty(t, FindAll(l, "goblin"))
}

// A string key works as well as an int one; nothing in List cares.
func TestStringKey(t *testing.T) {
	l := NewList(func(t *thing) string { return t.name })
	require.NoError(t, l.Add(&thing{1, "lizard"}))

	assert.True(t, errors.Is(l.Add(&thing{2, "lizard"}), ErrDuplicate))
	assert.True(t, l.Contains("lizard"))
}

// one added, two added, three pushed -> [three, one, two]
func TestPushGoesToTheFront(t *testing.T) {
	l := newTestList()
	one, two, three := &thing{1, "lizard"}, &thing{2, "lizard"}, &thing{3, "rat"}

	require.NoError(t, l.Add(one))
	require.NoError(t, l.Add(two))
	require.NoError(t, l.Push(three))

	assert.Equal(t, []*thing{three, one, two}, collect(l))
	got, found := Find(l, "rat")
	assert.True(t, found)
	assert.Same(t, three, got)
}

func TestPushDuplicateKeyIsErrDuplicate(t *testing.T) {
	l := newTestList()
	one := &thing{1, "lizard"}
	require.NoError(t, l.Add(one))

	err := l.Push(&thing{1, "rat"})

	assert.ErrorIs(t, err, ErrDuplicate)
	assert.Equal(t, []*thing{one}, collect(l), "a refused push leaves the list alone")
}
