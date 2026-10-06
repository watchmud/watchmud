package world

import (
	"slices"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
)

// TakeAfter is how long something dropped lies before a scavenging mob may
// take it: a moment to pick it back up. MaxCarried is all a mob will carry.
// Placeholders, like JunkAfter.
const (
	TakeAfter  = time.Minute
	MaxCarried = 10
)

// mobTakes is a script's me:take(): the mob picks up the longest-lying thing
// on its floor that it may -- left lying TakeAfter or longer, as the janitor
// counts it, and never a bag -- and keeps it. It answers what it took, or ""
// if nothing. What a mob carries goes into its corpse.
func (w *World) mobTakes(mob *mobile.Instance) string {
	room := w.mobileRoom(mob)
	if room == nil || mob.Inventory.Len() >= MaxCarried {
		return ""
	}
	found := slices.DeleteFunc(w.leftLying(room, time.Now(), TakeAfter),
		func(i *object.Instance) bool { return i.Contents != nil })
	if len(found) == 0 {
		return ""
	}
	item := slices.MinFunc(found, func(a, b *object.Instance) int { return a.DecaysAt.Compare(b.DecaysAt) })
	if err := object.Move(item, room.Inventory, mob.Inventory); err != nil {
		log.Error().Err(err).Str("mob", mob.Definition.Id).Msg("take")
		return ""
	}
	item.DecaysAt = time.Time{} // carried, not lying
	room.Send(event.Snatched{Mob: mob.Name(), Item: item.Definition.ShortDescription})
	return item.Definition.ShortDescription
}
