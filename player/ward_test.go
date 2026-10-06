package player

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
)

var wardClock = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

func TestWard_none(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "bob", &Recorder{})
	assert.Zero(t, p.Ward(wardClock))
	absorbed, broke := p.AbsorbWard(5, wardClock)
	assert.Zero(t, absorbed)
	assert.False(t, broke, "nothing to break")
}

func TestWard_absorbsUntilSpent(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "bob", &Recorder{})
	p.SetWard(10, wardClock, 30*time.Second)

	absorbed, broke := p.AbsorbWard(4, wardClock)
	assert.Equal(t, 4, absorbed)
	assert.False(t, broke)
	assert.Equal(t, 6, p.Ward(wardClock))

	absorbed, broke = p.AbsorbWard(9, wardClock)
	assert.Equal(t, 6, absorbed, "only what's left")
	assert.True(t, broke)
	assert.Zero(t, p.Ward(wardClock))
}

func TestWard_fades(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "bob", &Recorder{})
	p.SetWard(10, wardClock, 30*time.Second)

	assert.Equal(t, 10, p.Ward(wardClock.Add(29*time.Second)))
	assert.Zero(t, p.Ward(wardClock.Add(30*time.Second)))
	absorbed, _ := p.AbsorbWard(4, wardClock.Add(time.Minute))
	assert.Zero(t, absorbed)
}

// a second ward refreshes to the larger, and the clock starts again
func TestWard_refreshesNeverStacks(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "bob", &Recorder{})
	p.SetWard(10, wardClock, 30*time.Second)

	later := wardClock.Add(20 * time.Second)
	p.SetWard(6, later, 30*time.Second)
	assert.Equal(t, 10, p.Ward(later), "the larger, not 16")
	assert.Equal(t, 10, p.Ward(later.Add(29*time.Second)), "good for the new duration")

	p.AbsorbWard(8, later)
	p.SetWard(6, later, 30*time.Second)
	assert.Equal(t, 6, p.Ward(later), "a fresh one is bigger than what was left")
}

// a faded ward doesn't come back with the next one
func TestWard_fadedIsGone(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "bob", &Recorder{})
	p.SetWard(20, wardClock, 30*time.Second)

	later := wardClock.Add(time.Minute)
	p.SetWard(6, later, 30*time.Second)
	assert.Equal(t, 6, p.Ward(later))
}

func TestWard_reviveClearsIt(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "bob", &Recorder{})
	p.SetWard(10, wardClock, 30*time.Second)
	p.Revive()
	assert.Zero(t, p.Ward(wardClock))
}
