package loader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

// doorContent is lootContent's two zones, with a cellar and a pit in the
// caves joined both ways, and a door entry waiting to be hung between them.
func doorContent(t *testing.T, e doorEntry) (*Content, *spaces.Room, *spaces.Room) {
	t.Helper()
	c := lootContent(t)
	caves := c.Zones["caves"]
	cellar := spaces.NewRoom(caves, "cellar", "The Cellar", "")
	pit := spaces.NewRoom(caves, "pit", "The Pit", "")
	caves.AddRoom(cellar)
	caves.AddRoom(pit)
	cellar.Connect(rules.DirectionWest, pit)
	pit.Connect(rules.DirectionEast, cellar)
	c.pendingDoors = []pendingDoor{{"caves", "cellar", rules.DirectionWest, e}}
	return c, cellar, pit
}

// Declared on one side, the same door is on both; its key is "zone/id".
func TestHangDoors_bothSides(t *testing.T) {
	c, cellar, pit := doorContent(t, doorEntry{Name: "iron grate", Aliases: []string{"grate"},
		Closed: true, Locked: true, Key: "bone"})

	require.NoError(t, c.hangDoors())

	d := cellar.DoorTo(rules.DirectionWest)
	require.NotNil(t, d)
	assert.Same(t, d, pit.DoorTo(rules.DirectionEast))
	assert.Equal(t, "iron grate", d.Name)
	assert.Equal(t, "caves/bone", d.Key)
	assert.Equal(t, lock.State{Closed: true, Locked: true}, d.State)
	assert.Equal(t, []*spaces.Door{d}, c.Zones["caves"].Doors, "the declaring zone resets it")
}

func TestHangDoors_aPlainDoor(t *testing.T) {
	c, cellar, _ := doorContent(t, doorEntry{Closed: true})

	require.NoError(t, c.hangDoors())

	d := cellar.DoorTo(rules.DirectionWest)
	assert.Equal(t, "door", d.Name, "unnamed is a door")
	assert.Empty(t, d.Key)
}

func TestHangDoors_refuses(t *testing.T) {
	for name, e := range map[string]doorEntry{
		"locked but not closed": {Locked: true, Key: "bone"},
		"no key to open it":     {Closed: true, Locked: true},
		"object not defined":    {Closed: true, Key: "skull"},
		"zone not found":        {Closed: true, Key: "atlantis/key"},
	} {
		c, _, _ := doorContent(t, e)
		assert.ErrorContains(t, c.hangDoors(), name, name)
	}

	c, _, _ := doorContent(t, doorEntry{Closed: true})
	c.pendingDoors = append(c.pendingDoors, pendingDoor{"caves", "pit", rules.DirectionEast, doorEntry{}})
	assert.ErrorContains(t, c.hangDoors(), "one side only")
}
