package spaces

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/rules"
)

// out from the first room by id, a step north is y+1; a room nothing reaches
// is an island set apart
func TestLayGrid(t *testing.T) {
	z := NewZone("z", "Z", rules.ZoneResetNever, 0)
	a, b, c := NewRoom(z, "a", "A", ""), NewRoom(z, "b", "B", ""), NewRoom(z, "c", "C", "")
	lone := NewRoom(z, "lone", "Lone", "")
	for _, r := range []*Room{a, b, c, lone} {
		z.AddRoom(r)
	}
	a.Connect(rules.DirectionNorth, b)
	b.Connect(rules.DirectionSouth, a)
	b.Connect(rules.DirectionEast, c)
	c.Connect(rules.DirectionUp, a) // doesn't fit: a is two away

	misfits := z.LayGrid()

	assert.Equal(t, Coord{}, a.Grid)
	assert.Equal(t, Coord{0, 1, 0}, b.Grid)
	assert.Equal(t, Coord{1, 1, 0}, c.Grid)
	assert.Equal(t, Coord{3, 0, 0}, lone.Grid, "apart, east of everything")
	assert.Equal(t, []string{"z/c Up to z/a"}, misfits)
}
