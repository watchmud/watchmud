package bot

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/spaces"
)

// a square with two ways out, a lane east of it and a yard north
func smallAtlas() *atlas {
	m := newAtlas()
	m.see("Square", []string{"north", "east"})
	m.learn("Square", "east", "Lane")
	m.see("Lane", []string{"west", "south"})
	m.learn("Lane", "west", "Square")
	return m
}

func TestAtlas_untaken(t *testing.T) {
	m := smallAtlas()
	assert.Equal(t, []string{"north"}, m.untaken("Square"))
	assert.Equal(t, []string{"south"}, m.untaken("Lane"))
	m.learn("Square", "north", "Yard")
	assert.Empty(t, m.untaken("Square"))
	assert.Equal(t, 2, m.size(), "the rooms it has stood in: Yard waits until it gets there")
}

func TestAtlas_route(t *testing.T) {
	m := smallAtlas()
	assert.Equal(t, []string{"east"}, m.route("Square", func(r string) bool { return r == "Lane" }))
	assert.Equal(t, []string{}, m.route("Square", func(r string) bool { return r == "Square" }), "already there")
	assert.Nil(t, m.route("Square", func(r string) bool { return r == "Nowhere" }))
}

// a route never goes through a keepOut door, even one already walked
func TestAtlas_routeKeepsOut(t *testing.T) {
	m := newAtlas()
	m.learn("The Millpond", "west", "The Mill Yard")
	assert.Nil(t, m.route("The Millpond", func(r string) bool { return r == "The Mill Yard" }))
	m.see("The Millpond", []string{"west"})
	assert.Empty(t, m.untaken("The Millpond"))
}

// reachable is every room an explorer may reach from the start: no keepOut
// door, no door closed as content starts it.
func reachable(t *testing.T) int {
	t.Helper()
	content, err := loader.LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	start := content.Zones[content.Settings.Start.ZoneId].Rooms[content.Settings.Start.RoomId]
	seen := map[*spaces.Room]bool{start: true}
	queue := []*spaces.Room{start}
	for len(queue) > 0 {
		r := queue[0]
		queue = queue[1:]
		for _, ex := range r.Exits(false) {
			if kept(r.Name, strings.ToLower(ex.Direction.String())) || !r.Passable(ex.Direction) || seen[ex.Room] {
				continue
			}
			seen[ex.Room] = true
			queue = append(queue, ex.Room)
		}
	}
	return len(seen)
}

// Against the real world at a hundred times speed: it maps every room it may
// reach, and takes nothing.
func TestExplorer_mapsTheWorld(t *testing.T) {
	want := reachable(t)
	addr := startGame(t, 10*time.Millisecond)
	createCharacter(t, addr, "Mapper", "correcthorse")
	a := runAdventurer(t, addr, "Mapper", func(a *Adventurer) { a.cfg.Style = Explorer })

	ok := assert.Eventually(t, func() bool { return a.Stats().Mapped >= want }, 120*time.Second, 100*time.Millisecond)
	st := a.Stats()
	require.True(t, ok, "mapped %d of %d; stats %+v", st.Mapped, want, st)
	assert.Equal(t, want, st.Mapped, "nothing past a door it may not take")
	assert.Zero(t, st.Looted)
}

// A door seen open and shut since isn't a way any more: routed through, the
// explorer would try it, fail, and try it again for ever.
func TestAtlas_forgetsAShutDoor(t *testing.T) {
	m := newAtlas()
	m.see("Cellar", []string{"west"})
	m.learn("Cellar", "west", "Pit")
	m.see("Pit", []string{"east"})
	require.Equal(t, []string{"west"}, m.route("Cellar", func(r string) bool { return r == "Pit" }))

	m.see("Cellar", nil) // the grate is shut now
	assert.Nil(t, m.route("Cellar", func(r string) bool { return r == "Pit" }))
}
