package world

import (
	"strings"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/mobile"
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
	c := cast{caster: p, ability: a, power: grant.Power}
	if code := w.castTarget(&c, cmd.Target); code != "" {
		msg.Fail(code)
		return
	}
	if !p.SpendMana(a.Mana) {
		msg.Fail(event.NotEnoughMana)
		return
	}
	p.StartCooldown(a.Id, now.Add(a.Cooldown))
	effects[a.Id](w, c)
}

// castTarget resolves the target by the ability's kind into c. Every refusal
// here comes before anything is spent.
func (w *World) castTarget(c *cast, target string) event.ResultCode {
	switch c.ability.Target {
	case rules.TargetNone:
		return ""
	case rules.TargetSelf:
		c.target = c.caster
		return ""
	case rules.TargetFriend:
		if target == "" {
			c.target = c.caster
			return ""
		}
		p, found := w.playerRoom(c.caster).FindPlayer(target)
		if !found {
			return event.TargetNotFound
		}
		c.target = p
		return ""
	case rules.TargetFoe:
		return w.castFoe(c, target)
	}
	c.caster.Log().Error().Msgf("cast %s: target kind %q not handled yet", c.ability.Id, c.ability.Target)
	return event.InternalError
}

// castFoe finds the mob a foe ability is aimed at: the one named, or with no name,
// whoever the caster is fighting. Then the same refusals kill makes.
func (w *World) castFoe(c *cast, target string) event.ResultCode {
	room := w.playerRoom(c.caster)
	if target == "" {
		fight := w.fightLedger.GetFight(c.caster)
		if fight == nil {
			return event.NoFoe
		}
		mob, isMob := fight.Fightee.(*mobile.Instance)
		if !isMob {
			return event.NoFoe
		}
		c.foe = mob
	} else {
		mob, found := room.FindMobile(target)
		if !found {
			return event.TargetNotFound
		}
		c.foe = mob
	}
	if room.Flag(rules.RoomFlagNoFight) {
		return event.NoFightRoom
	}
	if c.foe.Definition.HasFlag(rules.MobileFlagPlayerCantFight) {
		return event.NoFight
	}
	return ""
}
