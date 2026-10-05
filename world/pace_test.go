package world

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A world ticking a hundred times fast has a clock to match, so a 60s
// cooldown is over in 0.6s of real time.
func TestSetPace(t *testing.T) {
	w, err := NewTestWorld()
	require.NoError(t, err)

	w.SetPace(100)
	start := w.now()
	time.Sleep(20 * time.Millisecond)
	elapsed := w.now().Sub(start)

	assert.GreaterOrEqual(t, elapsed, 2*time.Second, "20ms is at least 2s of game time")
	assert.Less(t, elapsed, 30*time.Second, "and not wildly more")
}
