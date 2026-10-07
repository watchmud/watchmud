// Package lock is a closable, lockable thing's state and rules: a door now, a
// chest next. It knows nothing of rooms, objects or players -- whether the one
// trying it holds the key is the caller's question, answered with a bool.
package lock

import "errors"

// Why an operation was refused. The world turns these into result codes.
var (
	ErrAlreadyOpen   = errors.New("already open")
	ErrAlreadyClosed = errors.New("already closed")
	ErrAlreadyLocked = errors.New("already locked")
	ErrNotLocked     = errors.New("not locked")
	ErrLocked        = errors.New("locked")
	ErrNotClosed     = errors.New("not closed")
	ErrNoKey         = errors.New("no key")
	ErrHasNoKeyhole  = errors.New("can't be locked")
)

// State is open or closed, locked or not. Locked implies closed.
type State struct {
	Closed bool
	Locked bool
}

// Lock is the state now, the state it starts in and resets to, and the key
// that fits it ("zone/object"; empty for a lock nothing locks).
type Lock struct {
	State
	Initial State
	Key     string
}

// New is a lock starting in this state.
func New(initial State, key string) *Lock {
	return &Lock{State: initial, Initial: initial, Key: key}
}

// Open it. A locked one stays shut.
func (l *Lock) Open() error {
	switch {
	case !l.Closed:
		return ErrAlreadyOpen
	case l.Locked:
		return ErrLocked
	}
	l.Closed = false
	return nil
}

// Close it.
func (l *Lock) Close() error {
	if l.Closed {
		return ErrAlreadyClosed
	}
	l.Closed = true
	return nil
}

// Lock it, with the key; only once it's closed.
func (l *Lock) Lock(hasKey bool) error {
	switch {
	case l.Key == "":
		return ErrHasNoKeyhole
	case l.Locked:
		return ErrAlreadyLocked
	case !l.Closed:
		return ErrNotClosed
	case !hasKey:
		return ErrNoKey
	}
	l.Locked = true
	return nil
}

// Unlock it, with the key. It stays closed.
func (l *Lock) Unlock(hasKey bool) error {
	switch {
	case l.Key == "":
		return ErrHasNoKeyhole
	case !l.Locked:
		return ErrNotLocked
	case !hasKey:
		return ErrNoKey
	}
	l.Locked = false
	return nil
}

// Reset puts it back how it started: a zone reset closes and locks what
// content said was closed and locked.
func (l *Lock) Reset() {
	l.State = l.Initial
}
