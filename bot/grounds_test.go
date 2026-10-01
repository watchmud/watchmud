package bot

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every ground's route and patrol, walked against the real content in a world
// with no pulses, so nothing picks a fight on the way round. A renamed room
// or a moved exit fails here, not in production.
func TestGrounds_walk(t *testing.T) {
	addr := startGame(t, time.Hour)
	createCharacter(t, addr, "Quillon", "correcthorse")
	for _, g := range grounds {
		t.Run(g.name, func(t *testing.T) {
			require.NotEmpty(t, g.route)
			require.NotEmpty(t, g.patrol)
			assert.Equal(t, g.route[len(g.route)-1].room, g.patrol[len(g.patrol)-1].room,
				"the patrol is a loop from where the route ends")
			assert.Equal(t, waitIn, g.route[0], "every route is out through the market, where bots wait")

			c := loginAs(t, addr, "Quillon", "correcthorse")
			require.NoError(t, recall(c))
			for _, s := range append(append([]step{}, g.route...), g.patrol...) {
				require.NoError(t, c.Send(s.dir))
				_, err := room(c, s.room)
				require.NoError(t, err, "going %s to %s", s.dir, s.room)
			}
			require.NoError(t, c.Send("quit"))
			require.NoError(t, c.ExpectClosed(5*time.Second))
		})
		// and the way out on foot, from every room on the patrol
		for i := range g.patrol {
			t.Run(fmt.Sprintf("%s, homeward from %s", g.name, g.patrol[i].room), func(t *testing.T) {
				c := loginAs(t, addr, "Quillon", "correcthorse")
				require.NoError(t, recall(c))
				way := append(append([]step{}, g.route...), g.patrol[:i+1]...)
				for _, s := range append(way, g.homeward(i)...) {
					require.NoError(t, c.Send(s.dir))
					_, err := room(c, s.room)
					require.NoError(t, err, "going %s to %s", s.dir, s.room)
				}
				require.NoError(t, c.Send("quit"))
				require.NoError(t, c.ExpectClosed(5*time.Second))
			})
		}
	}
}

// The short way round: back the way it came from near the loop's start, on
// round from near its end; then up the route, to the market.
func TestGround_homeward(t *testing.T) {
	g := &grounds[0]
	upTheRoute := []step{
		{"north", "southern path"},
		{"north", "Outside South Gate"},
		{"north", "South Gate"},
		{"north", "Market Square"},
	}

	assert.Equal(t, append([]step{{"east", "The Waystone"}}, upTheRoute...), g.homeward(0), "back from the millpond")
	assert.Equal(t, append([]step{
		{"north", "The Millpond"},
		{"east", "The Waystone"},
	}, upTheRoute...), g.homeward(3), "on round from the orchard, the second time through")
	assert.Equal(t, upTheRoute, g.homeward(len(g.patrol)-1), "already at the waystone")
	assert.Equal(t, append([]step{{"west", "The Waystone"}}, upTheRoute...), grounds[1].homeward(0), "back from the hedgerow")
}

// Broken gear stops counting toward power, so a bot in worn-out kit is power
// 0. It should still have somewhere to hunt, not idle in town forever.
func TestGrounds_bandCoversBrokenGear(t *testing.T) {
	found := false
	for _, g := range grounds {
		if g.minPower == 0 {
			found = true
		}
	}
	assert.True(t, found)
}

func TestGround_preyIn(t *testing.T) {
	g := &grounds[0]
	room := "The Red Barn\n A barn.\n[ Exits: n e ]\nA fat field rat noses through the straw.\nA fat field rat noses through the straw.\n"
	got := g.preyIn(room)
	require.Len(t, got, 1, "one of each kind; the second rat waits for the next lap")
	assert.Equal(t, "rat", got[0].keyword)
	assert.True(t, g.isPrey("field rat"))
	assert.False(t, g.isPrey("hedge-witch"), "her ring and censer are for players")
}

// A bot with no phrases is quiet, not broken; one with phrases has something
// for every moment.
func TestPhrases(t *testing.T) {
	for name, byMoment := range phrases {
		for _, m := range []moment{momentKill, momentTown, momentRest, momentDonate} {
			assert.NotEmpty(t, byMoment[m], "%s has nothing to say at moment %d", name, m)
		}
	}
	assert.Nil(t, phrases["nobody"][momentKill])
}
