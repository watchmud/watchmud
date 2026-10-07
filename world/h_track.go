package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

// handleTrack is "track bandit": the first step towards the nearest mob of
// that name, searching out from the player's room through open exits, within
// the zone -- a trail, not a map of the world. Run, then track again.
func (w *World) handleTrack(msg *gameserver.HandlerParameter, cmd command.Track) {
	if cmd.Target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	start := w.playerRoom(msg.Player)
	if _, here := start.FindMobile(cmd.Target); here {
		msg.Player.Send(event.Tracked{Target: cmd.Target, Here: true})
		return
	}
	type step struct {
		room  *spaces.Room
		first rules.Direction
	}
	seen := map[*spaces.Room]bool{start: true}
	queue := []step{{start, rules.DirectionNone}}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, ex := range s.room.Exits(true) {
			if seen[ex.Room] || !s.room.Passable(ex.Direction) {
				continue
			}
			seen[ex.Room] = true
			first := s.first
			if first == rules.DirectionNone {
				first = ex.Direction
			}
			if mob, found := ex.Room.FindMobile(cmd.Target); found {
				msg.Player.Send(event.Tracked{Target: mob.Name(), Direction: first})
				return
			}
			queue = append(queue, step{ex.Room, first})
		}
	}
	msg.Fail(event.NoTrail)
}
