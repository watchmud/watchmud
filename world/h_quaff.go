package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/rules"
)

// quaffCooldown is the key every potion's cooldown shares on the player: not
// an ability id, which can't start with a colon.
const quaffCooldown = ":quaff"

// handleQuaff is "quaff <potion>": the potion's ability, on the drinker, at
// the potion's power -- no mana, no gear, and the ability's own cooldown
// untouched -- and the potion is gone. One at a time, a whole
// rules.QuaffCooldown apart. Fine in a fight; that's what they're for.
func (w *World) handleQuaff(msg *gameserver.HandlerParameter, cmd command.Quaff) {
	p := msg.Player
	if cmd.Target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	target, err := parseTarget(cmd.Target)
	if err != nil {
		msg.Fail(event.ParseError)
		return
	}
	found := targetsIn(target, p.Inventory().All())
	if len(found) == 0 {
		msg.Fail(event.TargetNotFound)
		return
	}
	potion := found[0]
	a, known := w.content.Catalog.Abilities[potion.Definition.Quaff]
	if !known {
		msg.Fail(event.NotDrinkable)
		return
	}
	now := w.now()
	if now.Before(p.ReadyAt(quaffCooldown)) {
		msg.Fail(event.NotReady)
		return
	}
	if err := p.Inventory().Remove(potion); err != nil {
		log.Error().Err(err).Str("player", p.Name()).Msg("quaff")
		msg.Fail(event.Unknown)
		return
	}
	p.StartCooldown(quaffCooldown, now.Add(rules.QuaffCooldown))
	w.playerRoom(p).Send(event.Quaffed{Actor: p.Name(), Item: potion.Definition.ShortDescription})
	effects[a.Id](w, cast{caster: p, target: p, ability: a, power: potion.Power})
}
