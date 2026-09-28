// Package ordered provides a set that remembers the order things were added.
//
// A room's mobs, a room's floor and a player's inventory are all the same
// container: a lookup by id plus a stable order to show the player. Ranging a
// map gives neither, so a bare map makes "look" reshuffle and makes
// "kill lizard" pick a different lizard each time.
package ordered

import (
	"errors"
	"fmt"
	"iter"
	"slices"
)

var (
	// ErrDuplicate is returned by Add when the key is already in the list.
	ErrDuplicate = errors.New("already present")
	// ErrNotFound is returned by Remove when the key is not in the list.
	ErrNotFound = errors.New("not present")
)

// A List holds values of type T, keyed by K, in the order they were added.
//
// The key is supplied as a function rather than a constraint on T because the
// ids in this codebase are not uniform: mobile.Instance has an Id() method
// while object.Instance has an Id field, and a field cannot satisfy a method
// constraint. Pass a method expression ((*mobile.Instance).Id) or a closure.
type List[K comparable, T any] struct {
	byKey map[K]T
	order []T
	key   func(T) K
}

// NewList returns an empty List that identifies values by key.
func NewList[K comparable, T any](key func(T) K) *List[K, T] {
	return &List[K, T]{
		byKey: make(map[K]T),
		key:   key,
	}
}

// Add v to the end of the list. Adding a key that is already present is an
// error wrapping ErrDuplicate, and leaves the list unchanged.
func (l *List[K, T]) Add(v T) error {
	k := l.key(v)
	if _, exists := l.byKey[k]; exists {
		return fmt.Errorf("add %v: %w", k, ErrDuplicate)
	}
	l.byKey[k] = v
	l.order = append(l.order, v)
	return nil
}

// Push v onto the front of the list. Like Add, pushing a key that is already
// present is an error wrapping ErrDuplicate, and leaves the list unchanged.
func (l *List[K, T]) Push(v T) error {
	k := l.key(v)
	if _, exists := l.byKey[k]; exists {
		return fmt.Errorf("push %v: %w", k, ErrDuplicate)
	}
	l.byKey[k] = v
	l.order = slices.Insert(l.order, 0, v)
	return nil
}

// Remove v from the list, leaving the order of everything else intact.
// Removing a key that is not present is an error wrapping ErrNotFound.
func (l *List[K, T]) Remove(v T) error {
	return l.RemoveKey(l.key(v))
}

// RemoveKey removes the value with this key, leaving the order of everything
// else intact. Removing a key that is not present is an error wrapping
// ErrNotFound.
func (l *List[K, T]) RemoveKey(k K) error {
	if _, exists := l.byKey[k]; !exists {
		return fmt.Errorf("remove %v: %w", k, ErrNotFound)
	}
	delete(l.byKey, k)
	// slices.Delete rather than append(order[:i], order[i+1:]...): it zeroes
	// the tail, so the removed value isn't kept alive by the backing array.
	if i := slices.IndexFunc(l.order, func(other T) bool { return l.key(other) == k }); i >= 0 {
		l.order = slices.Delete(l.order, i, i+1)
	}
	return nil
}

// Get the value with this key.
func (l *List[K, T]) Get(k K) (T, bool) {
	v, exists := l.byKey[k]
	return v, exists
}

// Contains reports whether this key is in the list.
func (l *List[K, T]) Contains(k K) bool {
	_, exists := l.byKey[k]
	return exists
}

// All yields the values in the order they were added. Add and Remove
// invalidate a range in progress, the same as ranging a slice you are
// appending to.
func (l *List[K, T]) All() iter.Seq[T] {
	return slices.Values(l.order)
}

// AllExcept yields the values in the order they were added, skipping the one
// with this key.
func (l *List[K, T]) AllExcept(exclude K) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range l.order {
			if l.key(v) == exclude {
				continue
			}
			if !yield(v) {
				return
			}
		}
	}
}

// Slice returns a copy of the values, in the order they were added. Prefer
// All; this is for callers that need to hold on to the result.
func (l *List[K, T]) Slice() []T {
	return slices.Clone(l.order)
}

// Len is the number of values in the list.
func (l *List[K, T]) Len() int {
	return len(l.order)
}

// A Matcher answers whether a target string the player typed names it.
// Both object.Instance and mobile.Instance do, by delegating to their
// definition's name and aliases.
type Matcher interface {
	Matches(target string) bool
}

// Find the first value matching target, in the order things were added -- so
// two identical lizards are killed oldest first, and repeating the command
// gives the same answer.
//
// This is a function rather than a method because a method cannot introduce
// the Matcher constraint, and constraining List itself would shut out the
// values that have no Matches method.
func Find[K comparable, T Matcher](l *List[K, T], target string) (T, bool) {
	for _, v := range l.order {
		if v.Matches(target) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// FindAll values matching target, in the order they were added.
func FindAll[K comparable, T Matcher](l *List[K, T], target string) []T {
	var result []T
	for _, v := range l.order {
		if v.Matches(target) {
			result = append(result, v)
		}
	}
	return result
}
