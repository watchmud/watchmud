package spaces

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/rules"
)

func TestRoomExits_none(t *testing.T) {
	r := NewTestRoom("testing")
	exits := r.ExitString()
	assert.Equal(t, "None!", exits)
}

func TestRoomExits_all(t *testing.T) {
	r := NewTestRoom("testing")
	r.Connect(rules.DirectionNorth, r)
	r.Connect(rules.DirectionSouth, r)
	r.Connect(rules.DirectionEast, r)
	r.Connect(rules.DirectionWest, r)
	r.Connect(rules.DirectionUp, r)
	r.Connect(rules.DirectionDown, r)

	exits := r.ExitString()
	assert.Equal(t, "North, East, South, West, Up, Down", exits)
}

func TestRoomExits_some(t *testing.T) {
	r := NewTestRoom("test")
	r.Connect(rules.DirectionNorth, r)
	r.Connect(rules.DirectionEast, r)
	r.Connect(rules.DirectionUp, r)

	exits := r.ExitString()
	assert.Equal(t, "North, East, Up", exits)
}

func TestRoom_GetExitInfo(t *testing.T) {
	center := NewTestRoom("center")
	n := NewTestRoom("n")
	s := NewTestRoom("s")

	center.Connect(rules.DirectionNorth, n)
	n.Connect(rules.DirectionSouth, center)

	center.Connect(rules.DirectionSouth, s)
	s.Connect(rules.DirectionNorth, center)

	exitInfo := center.Exits(false)

	assert.Equal(t, 2, len(exitInfo))
	assert.Equal(t, rules.DirectionNorth, exitInfo[0].Direction)
	assert.Equal(t, rules.DirectionSouth, exitInfo[1].Direction)
}

func TestRoom_PickRandomDirection(t *testing.T) {
	center := NewTestRoom("center")
	// no rooms out
	dir := center.PickRandomDirection(false)
	assert.Equal(t, rules.DirectionNone, dir)

	n := NewTestRoom("n")
	center.Connect(rules.DirectionNorth, n)
	// one choice
	dir = center.PickRandomDirection(false)
	assert.Equal(t, rules.DirectionNorth, dir)

	// two choices
	s := NewTestRoom("s")
	center.Connect(rules.DirectionSouth, s)

	dir = center.PickRandomDirection(false)
	if !(dir == rules.DirectionNorth || dir == rules.DirectionSouth) {
		t.Errorf("expected NORTH or SOUTH but found %d", dir)
	}
}

func TestRoom_LimitToZone(t *testing.T) {
	zone1 := &Zone{Id: "zone1"}
	zone2 := &Zone{Id: "zone2"}
	center := NewTestRoom("center")
	center.Zone = zone1

	n := NewTestRoom("n")
	n.Zone = zone1
	s := NewTestRoom("s")
	s.Zone = zone2

	center.Connect(rules.DirectionNorth, n)
	n.Connect(rules.DirectionSouth, center)

	center.Connect(rules.DirectionSouth, s)
	s.Connect(rules.DirectionNorth, center)

	result := center.Exits(true)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, rules.DirectionNorth, result[0].Direction)
}

// A description says where the room is and where each exit leads, for a
// client that draws a map; a test room, with no zone, is just its id.
func TestDescription_where(t *testing.T) {
	z := NewZone("town", "The Town", rules.ZoneResetAlways, 0)
	a, b := NewRoom(z, "square", "Square", ""), NewRoom(z, "gate", "Gate", "")
	a.Connect(rules.DirectionSouth, b)

	d := a.DescriptionExcept(nil)
	assert.Equal(t, "town/square", d.Id)
	assert.Equal(t, "The Town", d.Area)
	assert.Equal(t, []event.ExitTo{{Direction: rules.DirectionSouth, To: "town/gate"}}, d.ExitTo)

	assert.Equal(t, "lonely", NewTestRoom("lonely").DescriptionExcept(nil).Id)
}
