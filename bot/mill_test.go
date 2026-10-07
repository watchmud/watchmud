package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Drowned Mill, walked against the real content in a world with no pulses:
// from Temple Square to every room the grate doesn't shut off, and back to
// the yard. A broken exit or a renamed room fails here.
func TestDrownedMill_walk(t *testing.T) {
	addr := startGame(t, time.Hour)
	createCharacter(t, addr, "Quillon", "correcthorse")
	c := loginAs(t, addr, "Quillon", "correcthorse")
	require.NoError(t, recall(c))

	way := append(append([]step{}, hollowfieldsRoute...),
		step{"west", "The Millpond"},
		step{"west", "The Mill Yard"},
		step{"west", "The Mill House"},
		step{"up", "The Grain Loft"},
	)
	for _, s := range way {
		require.NoError(t, c.Send(s.dir))
		_, err := room(c, s.room)
		require.NoError(t, err, "going %s to %s", s.dir, s.room)
	}
	// the grain bin is shut, and the strongbox key is in it
	require.NoError(t, c.Send("look in bin"))
	_, err := c.Expect(`It's closed\.`, 5*time.Second)
	require.NoError(t, err)
	require.NoError(t, c.Send("open bin"))
	_, err = c.Expect(`You open the grain bin\.`, 5*time.Second)
	require.NoError(t, err)
	require.NoError(t, c.Send("get key from bin"))
	_, err = c.Expect(`You get a small iron key from a grain bin\.`, 5*time.Second)
	require.NoError(t, err)
	require.NoError(t, c.Send("get bin"))
	_, err = c.Expect(`(?i)can't|cannot`, 5*time.Second)
	require.NoError(t, err, "a chest stays put")

	way = []step{
		{"down", "The Mill House"},
		{"down", "The Flooded Cellar"},
	}
	var last string
	for _, s := range way {
		require.NoError(t, c.Send(s.dir))
		text, err := room(c, s.room)
		require.NoError(t, err, "going %s to %s", s.dir, s.room)
		last = text
	}
	// the miller is behind a locked grate, and the key is a millhand's
	assert.Contains(t, last, "West (closed)")
	require.NoError(t, c.Send("west"))
	_, err = c.Expect(`The way is shut\.`, 5*time.Second)
	require.NoError(t, err)
	require.NoError(t, c.Send("unlock grate"))
	_, err = c.Expect(`You don't have the key\.`, 5*time.Second)
	require.NoError(t, err)

	for _, s := range []step{
		{"up", "The Mill House"},
		{"east", "The Mill Yard"},
		{"south", "The Sluice Gate"},
		{"west", "The Mill Race"},
		{"west", "The Reed Marsh"},
		{"east", "The Mill Race"},
		{"east", "The Sluice Gate"},
		{"north", "The Mill Yard"},
		{"east", "The Millpond"},
	} {
		require.NoError(t, c.Send(s.dir))
		_, err := room(c, s.room)
		require.NoError(t, err, "going %s to %s", s.dir, s.room)
	}
}
