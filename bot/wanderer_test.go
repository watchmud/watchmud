package bot

import (
	"context"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

func TestParseExits(t *testing.T) {
	assert.Equal(t, []string{"north", "east", "up"}, parseExits("North, East, Up"))
	assert.Empty(t, parseExits("None"))
	assert.Equal(t, []string{"north"}, parseExits("North, West (closed)"), "a wanderer doesn't open doors")
}

// never through a keepOut door, never straight back while there's another way
func TestPickExit(t *testing.T) {
	a, _ := adventurer(t)
	a.here = "The Edge of the Old Wood"
	for range 50 {
		// it came south from the bridge: north is back, south is the wolves
		assert.Equal(t, "west", a.pickExit([]string{"north", "west", "south"}, "south"))
	}

	a.here = "Smithy"
	assert.Equal(t, "east", a.pickExit([]string{"east"}, "west"), "a dead end: back is the only way")

	a.here = "The Edge of the Old Wood"
	assert.Equal(t, "", a.pickExit([]string{"south"}, ""), "nowhere it may go")
}

// It walks where the exits go, fights back when something attacks it, and
// takes nothing from what it killed.
func TestWander_walksAndFightsBack(t *testing.T) {
	a, g := adventurer(t)
	a.cfg.Style = Wanderer
	go func() { _, _ = a.wander(context.Background()) }()

	assert.Equal(t, "look", g.heard())
	g.say("Smithy\r\n A forge.\r\n[ Exits: East ]\r\n<100/100hp> ")
	assert.Equal(t, "east", g.heard(), "the only way out")
	g.say("Market Square\r\n A market.\r\n[ Exits: North, East, South, West ]\r\nrabbit hits you for 2 damage.\r\n<98/100hp> ")
	g.say("You hit rabbit for 3 damage.\r\nrabbit is dead!\r\n<98/100hp> ")

	next := g.heard()
	assert.Contains(t, []string{"north", "east", "south"}, next, "onward, not back west, and no looting")
	assert.Equal(t, 1, a.Stats().Steps)
	assert.Zero(t, a.Stats().Looted)
	assert.False(t, a.attacked)
}

// What makes a wanderer safe is a list of doors somebody wrote by hand. This
// walks the real content from the start room without them, and fails if a
// wanderer could reach anything aggressive it couldn't survive: add a door to
// keepOut, don't loosen the test.
func TestKeepOut_safe(t *testing.T) {
	const survivable = 2 // power: a power-1 bot holds its own a power above
	content, err := loader.LoadContent(os.DirFS("../content"))
	require.NoError(t, err)

	// every door named is real
	byName := map[string]*spaces.Room{}
	for _, z := range content.Zones {
		for _, r := range z.Rooms {
			byName[r.Name] = r
		}
	}
	for _, d := range keepOut {
		r, ok := byName[d.from]
		require.True(t, ok, "keepOut names a room that doesn't exist: %q", d.from)
		dir, err := rules.ParseDirection(d.dir)
		require.NoError(t, err)
		require.True(t, r.HasExit(dir), "%s has no exit %s", d.from, d.dir)
	}

	// where aggressive mobs too strong for it are put -- moonstruck ones too,
	// aggressive on a full-moon night -- and, for one that wanders, all of
	// its zone
	danger := map[*spaces.Room][]string{}
	for _, zid := range slices.Sorted(maps.Keys(content.Zones)) {
		z := content.Zones[zid]
		for _, cmd := range z.Commands {
			cm, ok := cmd.(spaces.CreateMobile)
			if !ok {
				continue
			}
			defZone := z
			if cm.ZoneId != "" {
				defZone = content.Zones[cm.ZoneId]
			}
			def := defZone.MobileDefinitions[cm.MobileDefinitionId]
			hostile := def != nil && (def.HasFlag(rules.MobileFlagAggressive) || def.HasFlag(rules.MobileFlagMoonstruck))
			if !hostile || def.Power <= survivable {
				continue
			}
			rooms := []*spaces.Room{z.Rooms[cm.RoomId]}
			if def.Wandering.CanWander {
				rooms = slices.Collect(maps.Values(z.Rooms))
			}
			for _, r := range rooms {
				danger[r] = append(danger[r], def.Name)
			}
		}
	}
	require.NotEmpty(t, danger, "nothing dangerous at all? then this test proves nothing")

	start := content.Zones[content.Settings.Start.ZoneId].Rooms[content.Settings.Start.RoomId]
	seen := map[*spaces.Room]bool{start: true}
	queue := []*spaces.Room{start}
	for len(queue) > 0 {
		r := queue[0]
		queue = queue[1:]
		assert.Empty(t, danger[r], "a wanderer can reach %s (%s)", r.Name, r.Zone.Id)
		for _, ex := range r.Exits(false) {
			if kept(r.Name, strings.ToLower(ex.Direction.String())) {
				continue
			}
			if !seen[ex.Room] {
				seen[ex.Room] = true
				queue = append(queue, ex.Room)
			}
		}
	}
	assert.Greater(t, len(seen), 20, "and somewhere worth wandering")
}

// Against the real world at a hundred times speed: it keeps walking, and
// never takes anything.
func TestWanderer_roams(t *testing.T) {
	addr := startGame(t, 10*time.Millisecond)
	createCharacter(t, addr, "Rover", "correcthorse")
	a := runAdventurer(t, addr, "Rover", func(a *Adventurer) { a.cfg.Style = Wanderer })

	ok := assert.Eventually(t, func() bool { return a.Stats().Steps >= 30 }, 60*time.Second, 50*time.Millisecond)
	st := a.Stats()
	require.True(t, ok, "stats: %+v", st)
	assert.Zero(t, st.Looted)
	assert.Zero(t, st.Donations)
}
