package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Drowned Mill, walked against the real content in a world with no pulses:
// from Temple Square to every room and back to the yard, with the miller at
// the bottom of it. A broken exit or a renamed room fails here.
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
		step{"down", "The Mill House"},
		step{"down", "The Flooded Cellar"},
		step{"west", "The Wheel Pit"},
	)
	var last string
	for _, s := range way {
		require.NoError(t, c.Send(s.dir))
		text, err := room(c, s.room)
		require.NoError(t, err, "going %s to %s", s.dir, s.room)
		last = text
	}
	assert.Contains(t, last, "The Drowned Miller rises from the black water")

	for _, s := range []step{
		{"east", "The Flooded Cellar"},
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
