package spaces

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/rules"
)

// cellar -- west, through an iron grate -- pit, and the same grate on the way back
func doorway() (cellar, pit *Room, grate *Door) {
	z := NewZone("mill", "The Drowned Mill", rules.ZoneResetAlways, 0)
	cellar, pit = NewRoom(z, "cellar", "The Flooded Cellar", ""), NewRoom(z, "pit", "The Wheel Pit", "")
	cellar.Connect(rules.DirectionWest, pit)
	pit.Connect(rules.DirectionEast, cellar)
	grate = NewDoor("iron grate", []string{"bars"}, lock.New(lock.State{Closed: true, Locked: true}, "mill/grate_key"))
	cellar.SetDoor(rules.DirectionWest, grate)
	pit.SetDoor(rules.DirectionEast, grate)
	z.Doors = append(z.Doors, grate)
	return
}

func TestDoor_closedIsNotPassable(t *testing.T) {
	cellar, pit, grate := doorway()

	assert.True(t, cellar.HasExit(rules.DirectionWest), "the exit is there")
	assert.False(t, cellar.Passable(rules.DirectionWest), "but shut")
	assert.Equal(t, "West (closed)", cellar.ExitString())

	grate.Locked, grate.Closed = false, false
	assert.True(t, cellar.Passable(rules.DirectionWest))
	assert.True(t, pit.Passable(rules.DirectionEast), "one door, both sides")
	assert.Equal(t, "West", cellar.ExitString())
}

func TestDoor_mobsDontWanderThroughIt(t *testing.T) {
	cellar, _, _ := doorway()
	assert.Equal(t, rules.DirectionNone, cellar.PickRandomDirection(false))
}

func TestFindDoor(t *testing.T) {
	cellar, _, grate := doorway()

	for _, target := range []string{"west", "w", "iron grate", "grate", "Iron", "bars", "door"} {
		dir, d, ok := cellar.FindDoor(target)
		if assert.True(t, ok, target) {
			assert.Equal(t, rules.DirectionWest, dir, target)
			assert.Same(t, grate, d, target)
		}
	}
	for _, target := range []string{"east", "gate", ""} {
		_, _, ok := cellar.FindDoor(target)
		assert.False(t, ok, target)
	}
}

func TestZoneReset_resetsItsDoors(t *testing.T) {
	cellar, _, grate := doorway()
	grate.Locked, grate.Closed = false, false

	cellar.Zone.Reset(NewOccupancy())

	assert.Equal(t, lock.State{Closed: true, Locked: true}, grate.State)
}
