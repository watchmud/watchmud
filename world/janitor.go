package world

import (
	"slices"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

// JunkAfter is how long something a player dropped lies on a floor before a
// sweeper may count it as junk: long enough to drop a thing for a friend and
// have them pick it up. A placeholder, like DroppedDecay.
const JunkAfter = 5 * time.Minute

// junk is what a sweeper may take off room's floor at now: what a player
// dropped and left lying JunkAfter or longer.
func (w *World) junk(room *spaces.Room, now time.Time) []*object.Instance {
	return w.leftLying(room, now, JunkAfter)
}

// leftLying is what a player dropped on room's floor and left there for at
// least after: never a corpse or anything else that can't be taken, never what
// a reset put down, and nothing at all in the donation room -- leaving things
// there is what it's for. What a mob may help itself to.
func (w *World) leftLying(room *spaces.Room, now time.Time, after time.Duration) []*object.Instance {
	if d := w.content.Settings.Donation; room.Zone != nil && room.Zone.Id == d.ZoneId && room.Id == d.RoomId {
		return nil
	}
	var found []*object.Instance
	for item := range room.Inventory.All() {
		if item.DecaysAt.IsZero() || item.Definition.NoTake() {
			continue
		}
		droppedAt := item.DecaysAt.Add(-rules.DroppedDecay)
		if now.Sub(droppedAt) >= after {
			found = append(found, item)
		}
	}
	return found
}

// junkHere is a script's me:junk().
func (w *World) junkHere(mob *mobile.Instance) int {
	room := w.mobileRoom(mob)
	if room == nil {
		return 0
	}
	return len(w.junk(room, time.Now()))
}

// sweep is a script's me:sweep(n): up to n pieces of junk off the mob's
// floor and out of the world, oldest first, the room told of each.
func (w *World) sweep(mob *mobile.Instance, n int) int {
	room := w.mobileRoom(mob)
	if room == nil {
		return 0
	}
	found := w.junk(room, time.Now())
	// the longest-lying goes first
	slices.SortStableFunc(found, func(a, b *object.Instance) int { return a.DecaysAt.Compare(b.DecaysAt) })
	swept := 0
	for _, item := range found {
		if swept == n {
			break
		}
		if err := room.Inventory.Remove(item); err != nil {
			log.Error().Err(err).Str("mob", mob.Definition.Id).Msg("sweep")
			continue
		}
		room.Send(event.Swept{Sweeper: mob.Name(), Item: item.Definition.ShortDescription})
		swept++
	}
	return swept
}
