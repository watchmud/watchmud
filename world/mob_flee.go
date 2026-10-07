package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
)

// mobFlees is a script's me:flee(): the mob breaks off every fight it's in
// and runs through an open exit that keeps it in its own zone, picked through
// w.roller, the room told as it is of a player fleeing. It answers whether
// it got away -- a mob with nowhere to run stays and fights.
func (w *World) mobFlees(mob *mobile.Instance) bool {
	room := w.mobileRoom(mob)
	if room == nil || !w.fightLedger.InFight(mob) {
		return false
	}
	var ways []int
	exits := room.Exits(true)
	for i, ex := range exits {
		if room.Passable(ex.Direction) {
			ways = append(ways, i)
		}
	}
	if len(ways) == 0 {
		return false
	}
	pick, err := w.roller.IntN(len(ways))
	if err != nil {
		log.Error().Err(err).Str("mob", mob.Definition.Id).Msg("flee")
		return false
	}
	ex := exits[ways[pick]]
	room.Send(event.Fled{Who: mob.Name()})
	w.fightLedger.EndAllFightsWith(mob.Id())
	w.moveMobile(mob, ex.Direction, ex.Room)
	return true
}
