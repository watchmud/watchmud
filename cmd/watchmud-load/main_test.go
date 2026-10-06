package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/player"
)

// every name it makes is one the game takes, and none twice
func TestNameFor(t *testing.T) {
	seen := map[string]bool{}
	for i := range 2000 {
		n := nameFor(i)
		canon, err := player.CanonicalName(n)
		require.NoError(t, err, n)
		require.False(t, seen[canon], "%s twice", canon)
		seen[canon] = true
	}
}

// the pieces against a real game, briefly: create from a spread address,
// log in, look and time it
func TestCreateAndProbe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr, err := startGame(ctx, "../../content", 10*time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, create(ctx, addr, "127.0.0.9", nameFor(0)))

	// long enough to log in on a busy machine -- make check runs every
	// package at once -- and then look a few times
	pctx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	took, failed := probe(pctx, addr, "127.0.0.9", nameFor(0))
	assert.Zero(t, failed)
	assert.GreaterOrEqual(t, len(took), 2)
}
