package lock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLock_openAndClose(t *testing.T) {
	l := New(State{Closed: true}, "")

	assert.NoError(t, l.Open())
	assert.False(t, l.Closed)
	assert.ErrorIs(t, l.Open(), ErrAlreadyOpen)
	assert.NoError(t, l.Close())
	assert.ErrorIs(t, l.Close(), ErrAlreadyClosed)
}

func TestLock_lockedStaysShut(t *testing.T) {
	l := New(State{Closed: true, Locked: true}, "mill/grate_key")

	assert.ErrorIs(t, l.Open(), ErrLocked)
	assert.ErrorIs(t, l.Unlock(false), ErrNoKey, "not without the key")
	assert.NoError(t, l.Unlock(true))
	assert.True(t, l.Closed, "unlocked isn't open")
	assert.ErrorIs(t, l.Unlock(true), ErrNotLocked)
	assert.NoError(t, l.Open())
}

func TestLock_lockingNeedsItClosedAndTheKey(t *testing.T) {
	l := New(State{}, "mill/grate_key")

	assert.ErrorIs(t, l.Lock(true), ErrNotClosed)
	assert.NoError(t, l.Close())
	assert.ErrorIs(t, l.Lock(false), ErrNoKey)
	assert.NoError(t, l.Lock(true))
	assert.ErrorIs(t, l.Lock(true), ErrAlreadyLocked)
}

// a plain door: no key fits it, so it never locks
func TestLock_noKeyhole(t *testing.T) {
	l := New(State{Closed: true}, "")

	assert.ErrorIs(t, l.Lock(true), ErrHasNoKeyhole)
	assert.ErrorIs(t, l.Unlock(true), ErrHasNoKeyhole)
}

func TestLock_reset(t *testing.T) {
	l := New(State{Closed: true, Locked: true}, "mill/grate_key")
	_ = l.Unlock(true)
	_ = l.Open()

	l.Reset()

	assert.Equal(t, State{Closed: true, Locked: true}, l.State)
}
