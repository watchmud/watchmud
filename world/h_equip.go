package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/rules"
)

// handleEquip puts something from the pack into a named slot -- wield's
// hand. What it picks is wear's choice, held to that slot: see pickToWear.
func (w *World) handleEquip(msg *gameserver.HandlerParameter, cmd command.Equip) {
	if cmd.Slot <= rules.SlotNone {
		msg.Fail(event.NoSlotGiven)
		return
	}
	if cmd.Target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	target, err := parseTarget(cmd.Target)
	if err != nil {
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("target", cmd.Target).Msg("equip: can't parse target")
		msg.Fail(event.ParseError)
		return
	}
	item, code := pickToWear(msg.Player, target, cmd.Slot)
	if code != "" {
		msg.Fail(code)
		return
	}
	msg.Player.Equipment().Equip(cmd.Slot, item)
	msg.Player.Send(event.Equipped{Item: item.Definition.ShortDescription})
}
