package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The town, as the socialite and the shopkeeper give directions to it: the
// store east of Market Square, the smithy west, the donation room east of
// Temple Square. An exit that goes missing sends the newest players astray.
func TestTown_walk(t *testing.T) {
	addr := startGame(t, time.Hour)
	createCharacter(t, addr, "Quillon", "correcthorse")
	c := loginAs(t, addr, "Quillon", "correcthorse")
	require.NoError(t, recall(c))

	for _, s := range []step{
		{"east", "Donation Room"},
		{"west", "Temple Square"},
		{"south", "Market Square"},
		{"east", "General Store"},
		{"west", "Market Square"},
		{"west", "Smithy"},
	} {
		require.NoError(t, c.Send(s.dir))
		_, err := room(c, s.room)
		require.NoError(t, err, "going %s to %s", s.dir, s.room)
	}
}
