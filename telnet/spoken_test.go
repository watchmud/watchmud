package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/rules"
)

func sayIt(t *testing.T, msg any) string {
	t.Helper()
	s, ok := spoken(msg, "testdood")
	assert.True(t, ok, "screen-reader mode has words for %T", msg)
	return plain(s)
}

func TestSpoken_prompt(t *testing.T) {
	assert.Equal(t, "health 80 of 100, mana 20 of 30. ",
		sayIt(t, event.Prompt{CurrentHealth: 80, MaxHealth: 100, CurrentMana: 20, MaxMana: 30}))
}

// The map is a list in words, never the picture.
func TestSpoken_map(t *testing.T) {
	m := event.Map{Zone: "The Drowned Mill", Rooms: []event.MapRoom{
		{X: -1, Y: 2, Name: "The Loft"},
		{X: 0, Y: 0, Name: "The Flooded Cellar", Here: true, Exits: []event.MapExit{
			{Direction: rules.DirectionWest, Leads: "The Wheel Pit", Closed: true},
			{Direction: rules.DirectionUp, Leads: "The Mill House"}}},
		{X: -1, Y: 0, Name: "The Wheel Pit"},
	}}
	assert.Equal(t, "You are at The Flooded Cellar, in The Drowned Mill.\n"+
		"From here: west to The Wheel Pit, through a closed door; up to The Mill House.\n"+
		"2 rooms nearby, on this level:\n"+
		"The Loft, 2 north and 1 west.\n"+
		"The Wheel Pit, 1 west.\n", sayIt(t, m))
}

func TestSpoken_exits(t *testing.T) {
	assert.Equal(t, "2 ways out: north to The Waystone; east to The Shed, through a closed door.\n",
		sayIt(t, event.Exits{Exits: []event.Exit{{Direction: rules.DirectionNorth, RoomName: "The Waystone"},
			{Direction: rules.DirectionEast, RoomName: "The Shed", Closed: true}}}))
	assert.Equal(t, "There's no way out of here.\n", sayIt(t, event.Exits{}))
}

// The exits line loses its brackets; the rest of the room is as it was.
func TestSpoken_room(t *testing.T) {
	assert.Equal(t, "Temple Square\nExits: North, South.\n",
		sayIt(t, event.RoomDescription{Name: "Temple Square", Exits: "North, South"}))
}

// A list says how long it is before it starts, and fractions are words.
func TestSpoken_lists(t *testing.T) {
	got := sayIt(t, event.GroupList{Members: []event.GroupMember{
		{Name: "Ann", Leader: true, Health: 97, MaxHealth: 100, Mana: 80, MaxMana: 100, Room: "Temple Square"},
		{Name: "Bob", Health: 5, MaxHealth: 100, Room: "Temple Square"}}})
	assert.Contains(t, got, "2 members in your group.\nGroup\n")
	assert.Contains(t, got, "97 of 100 health")
	assert.Contains(t, got, "80 of 100 mana")
	assert.NotContains(t, got, "/")

	assert.Equal(t, "You aren't carrying anything.\n", sayIt(t, event.Inventory{}), "an empty list says so itself")
	assert.Contains(t, sayIt(t, event.Who{Players: []event.WhoEntry{{PlayerName: "Ann"}}}), "One player online.\n")
}

func TestSpoken_onlyWhereItDiffers(t *testing.T) {
	_, ok := spoken(event.Pong{Target: "testdood"}, "testdood")
	assert.False(t, ok)
}
