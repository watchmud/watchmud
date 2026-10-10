package world

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

// handleGoals names the zones a player's power fits, and the one just above:
// the ladder the manifest's power bands already describe, read as a path.
// Derived on every read -- no quest state, nothing stored (LEVELS.md).
func (w *World) handleGoals(msg *gameserver.HandlerParameter, cmd command.Goals) {
	power := msg.Player.Power()
	goals := event.Goals{Power: power, From: w.StartRoom.Name}
	nextMin := 0
	for _, z := range w.ladder() {
		switch {
		case z.Power.Min <= power && power <= z.Power.Max:
			goals.Fits = append(goals.Fits, w.goalZone(z))
		case z.Power.Min > power && (nextMin == 0 || z.Power.Min == nextMin):
			nextMin = z.Power.Min
			goals.Next = append(goals.Next, w.goalZone(z))
		}
	}
	msg.Player.Send(goals)
}

// ladder is every zone built for a power band, lowest first. A zone with no
// band (the town, the void) is no step on it.
func (w *World) ladder() []*spaces.Zone {
	var zones []*spaces.Zone
	for z := range w.Zones() {
		if z.Power.Max > 0 {
			zones = append(zones, z)
		}
	}
	slices.SortFunc(zones, func(a, b *spaces.Zone) int {
		return cmp.Or(cmp.Compare(a.Power.Min, b.Power.Min), cmp.Compare(a.Power.Max, b.Power.Max), cmp.Compare(a.Id, b.Id))
	})
	return zones
}

func (w *World) goalZone(z *spaces.Zone) event.GoalZone {
	return event.GoalZone{Name: z.Name, Min: z.Power.Min, Max: z.Power.Max, Route: speedwalk(routeTo(w.StartRoom, z))}
}

// routeTo is the shortest walk from a room into a zone, by every exit -- a
// door on the way is one to open, not a wall. Nil if the room is already in
// it, or there's no way.
func routeTo(from *spaces.Room, z *spaces.Zone) []rules.Direction {
	if from.Zone == z {
		return nil
	}
	type step struct {
		room *spaces.Room
		path []rules.Direction
	}
	seen := map[*spaces.Room]bool{from: true}
	queue := []step{{from, nil}}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, ex := range s.room.Exits(false) {
			if seen[ex.Room] {
				continue
			}
			seen[ex.Room] = true
			path := append(slices.Clone(s.path), ex.Direction)
			if ex.Room.Zone == z {
				return path
			}
			queue = append(queue, step{ex.Room, path})
		}
	}
	return nil
}

// speedwalk writes a walk the way MUD players do: "5s 2w d".
func speedwalk(path []rules.Direction) string {
	var parts []string
	for i := 0; i < len(path); {
		n := 1
		for i+n < len(path) && path[i+n] == path[i] {
			n++
		}
		part := path[i].Abbrev()
		if n > 1 {
			part = strconv.Itoa(n) + part
		}
		parts = append(parts, part)
		i += n
	}
	return strings.Join(parts, " ")
}
