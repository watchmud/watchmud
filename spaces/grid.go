package spaces

import (
	"fmt"
	"maps"
	"slices"

	"github.com/watchmud/watchmud/rules"
)

// Coord is where a room sits on its zone's map: x east, y north, z up.
type Coord struct{ X, Y, Z int }

// step is where one move in each direction goes on the grid.
var step = map[rules.Direction]Coord{
	rules.DirectionNorth: {0, 1, 0},
	rules.DirectionSouth: {0, -1, 0},
	rules.DirectionEast:  {1, 0, 0},
	rules.DirectionWest:  {-1, 0, 0},
	rules.DirectionUp:    {0, 0, 1},
	rules.DirectionDown:  {0, 0, -1},
}

func (c Coord) plus(d Coord) Coord { return Coord{c.X + d.X, c.Y + d.Y, c.Z + d.Z} }

// LayGrid gives every room in the zone its Grid, walking exits within the zone
// out from the first room by id: a step north is y+1, and so on. A room no
// exit reaches, or one whose spot is taken, starts an island of its own, set
// apart east of everything placed so far. It answers each exit that doesn't
// fit the grid -- a step that lands somewhere else, or on another room -- for
// a test to hold the content to; a mapper is only as good as these are few.
//
// A client maps from these (GMCP Room.Info) instead of guessing where a room
// it jumped to -- a recall, a login, a death -- goes.
func (z *Zone) LayGrid() (misfits []string) {
	placed := map[*Room]bool{}
	taken := map[Coord]*Room{}
	maxX := 0
	put := func(r *Room, at Coord) {
		r.Grid, placed[r], taken[at] = at, true, r
		maxX = max(maxX, at.X)
	}
	for _, id := range slices.Sorted(maps.Keys(z.Rooms)) {
		start := z.Rooms[id]
		if placed[start] {
			continue
		}
		at := Coord{}
		if len(placed) > 0 {
			at = Coord{X: maxX + 2}
		}
		for taken[at] != nil {
			at.X++
		}
		put(start, at)
		queue := []*Room{start}
		for len(queue) > 0 {
			r := queue[0]
			queue = queue[1:]
			for _, ex := range r.Exits(true) {
				d, ok := step[ex.Direction]
				if !ok {
					continue
				}
				want := r.Grid.plus(d)
				switch {
				case placed[ex.Room]:
					if ex.Room.Grid != want {
						misfits = append(misfits, fmt.Sprintf("%s %s to %s", r.ref(), ex.Direction, ex.Room.ref()))
					}
				case taken[want] != nil:
					misfits = append(misfits, fmt.Sprintf("%s %s to %s: the spot is %s's", r.ref(), ex.Direction, ex.Room.ref(), taken[want].ref()))
				default:
					put(ex.Room, want)
					queue = append(queue, ex.Room)
				}
			}
		}
	}
	return misfits
}
