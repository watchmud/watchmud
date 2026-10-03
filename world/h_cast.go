package world

import (
	"strings"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// handleCast uses an ability the player's equipped gear grants. Every
// refusal comes before anything is spent; once the checks pass the cast happens
// and is paid for, even if it turns out to do nothing (healing someone who isn't hurt).
func (w *World) handleCast(msg *gameserver.HandlerParameter, cmd command.Cast) {
	p := msg.Player
	if cmd.Ability == "" {
		msg.Fail(event.NoTarget)
		return
	}
	a, known := w.content.Catalog.Abilities[strings.ToLower(cmd.Ability)]
	if !known {
		msg.Fail(event.UnknownAbility)
		return
	}
	grant, granted := p.Equipment().Abilities()[a.Id]
	if !granted {
		msg.Fail(event.NotGranted)
		return
	}
	now := w.now()
	if now.Before(p.ReadyAt(a.Id)) {
		msg.Fail(event.NotReady)
		return
	}
	target, code := w.castTarget(p, a, cmd.Target)
	if code != "" {
		msg.Fail(code)
		return
	}
	if !p.SpendMana(a.Mana) {
		msg.Fail(event.NotEnoughMana)
		return
	}
	p.StartCooldown(a.Id, now.Add(a.Cooldown))
	effects[a.Id](w, cast{caster: p, target: target, ability: a, power: grant.Power})
}

// castTarget resolves the target by the ability's kind. Only the kinds an
// ability uses so far are here; foe arrives with the first offensive one.
func (w *World) castTarget(caster *player.Player, a *rules.Ability, target string) (*player.Player, event.ResultCode) {
	switch a.Target {
	case rules.TargetSelf:
		return caster, ""
	case rules.TargetFriend:
		if target == "" {
			return caster, ""
		}
		if p, found := w.playerRoom(caster).FindPlayer(target); found {
			return p, ""
		}
		return nil, event.TargetNotFound
	}
	caster.Log().Error().Msgf("cast %s: target kind %q not handled yet", a.Id, a.Target)
	return nil, event.InternalError
}
