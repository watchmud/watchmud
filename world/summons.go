package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/spaces"
)

// summon calls up count of def into summoner's room, each going for a player there,
// and answers how many came. The caps and the "may it summon that?" check are the
// script runtime's; this is the engine doing what it's asked.
func (w *World) summon(summoner *mobile.Instance, def *mobile.Definition, count int) int {
	room := w.mobileRoom(summoner)
	summons := make([]*mobile.Instance, 0, count)
	for range count {
		mob := mobile.NewInstance(def)
		mob.Summoner = summoner
		w.PlaceMobile(mob, room)
		summons = append(summons, mob)
	}
	// All of them, then the fights: the room reads "the King calls up two skeletons"
	// before it reads anything they do.
	room.Send(event.Summoned{Summoner: summoner.Name(), Name: def.Name, Count: count})
	for _, mob := range summons {
		w.summonAttacks(mob, room)
	}
	return count
}

// summonAttacks sends a summon at a player in the room, picked at random, so
// the guard spreads over the group instead of piling onto the tank. A wizard with
// nohassle isn't picked, as aggro doesn't pick them. With nobody to pick,
// the summon stands; it crumbles with the rest.
func (w *World) summonAttacks(mob *mobile.Instance, room *spaces.Room) {
	var targets []*player.Player
	for _, p := range room.Players() {
		if p.IsWizard() && p.NoHassle() {
			continue
		}
		targets = append(targets, p)
	}
	if len(targets) == 0 {
		return
	}
	i, err := w.roller.IntN(len(targets))
	if err != nil {
		log.Error().Err(err).Msgf("summon: %s picking a target", mob.Definition.Id)
		return
	}
	if err = w.startFight(mob, targets[i]); err != nil {
		log.Error().Err(err).Msgf("summon: %s starting its fight", mob.Definition.Id)
	}
}

// liveSummons is how many of summoner's summons are in the world.
func (w *World) liveSummons(summoner *mobile.Instance) int {
	n := 0
	for _, mob := range w.Mobiles() {
		if mob.Summoner == summoner {
			n++
		}
	}
	return n
}

// crumble takes a summon out of the world: no corpse, no loot, no coins. Its
// fights end the way a death's do, so whatever it was holding retargets.
func (w *World) crumble(mob *mobile.Instance) {
	if room := w.roomOf(mob); room != nil {
		room.Notify(event.Crumbled{Name: mob.Name()})
	}
	w.fightLedger.EndAllFightsWith(mob.Id())
	w.RemoveMobile(mob)
}

// crumbleSummonsOf crumbles everything summoner called up.
func (w *World) crumbleSummonsOf(summoner *mobile.Instance) {
	for _, mob := range w.Mobiles() {
		if mob.Summoner == summoner {
			w.crumble(mob)
		}
	}
}

// sweepSummons crumbles every summon whose summoner's fight is over:
// the summoner gone from the world, or here and fighting nobody. One check,
// run every round, covers every way a fight ends - fled, wiped, ways not written yet.
// A death doesn't wait for it: combatantDied crumbles them on the spot.
func (w *World) sweepSummons() {
	for _, mob := range w.Mobiles() {
		s := mob.Summoner
		if s == nil {
			continue
		}
		if w.roomOf(s) == nil || !w.fightLedger.InFight(s) {
			w.crumble(mob)
		}
	}
}
