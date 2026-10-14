package world

import (
	"cmp"
	"slices"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// mapReach is how many steps out from the player "map" shows, each way.
const mapReach = 3

// handleMap is the rooms near the player, from the zone's grid: those on the
// same level, within mapReach steps east-west and north-south. Every room of
// the zone, not only those visited -- nobody's travels are stored.
func (w *World) handleMap(msg *gameserver.HandlerParameter, cmd command.Map) {
	here := w.playerRoom(msg.Player)
	m := event.Map{Zone: here.Zone.Name}
	for _, r := range here.Zone.Rooms {
		dx, dy := r.Grid.X-here.Grid.X, r.Grid.Y-here.Grid.Y
		if r.Grid.Z != here.Grid.Z || abs(dx) > mapReach || abs(dy) > mapReach {
			continue
		}
		mr := event.MapRoom{X: dx, Y: dy, Name: r.Name, Here: r == here}
		for _, ex := range r.Exits(false) {
			mr.Exits = append(mr.Exits, event.MapExit{Direction: ex.Direction, Leads: ex.Room.Name,
				Closed: !ex.Passable(), Away: ex.Room.Zone != r.Zone})
		}
		m.Rooms = append(m.Rooms, mr)
	}
	// north to south, west to east: the order a reader would take them in
	slices.SortFunc(m.Rooms, func(a, b event.MapRoom) int {
		return cmp.Or(cmp.Compare(b.Y, a.Y), cmp.Compare(a.X, b.X))
	})
	msg.Player.Send(m)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
