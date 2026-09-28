package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/rules"
)

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

	// TODO need to sort this by priority of how we count "2.x"
	// and make sense of other info in the target data structure
	// TODO other soring things
	objectsToEquip := msg.Player.Inventory().FindAll(target.Name)
	if len(objectsToEquip) == 0 {
		// you don't have one
		msg.Fail(event.TargetNotFound)
		return
	}

	// TODO for now, use the first one returned
	objectToEquip := objectsToEquip[0]

	// do you already have something equipped in that location?
	if msg.Player.Equipment().Equipped(cmd.Slot) {
		msg.Fail(event.LocationInUse)
		return
	}

	// can this object be equiped there?
	if cmd.Slot != objectToEquip.Definition.EquipmentSlot {
		msg.Fail(event.CantWearThere)
		return
	}
	// success
	msg.Player.Equipment().Equip(cmd.Slot, objectToEquip)
	msg.Player.Send(event.Equipped{})
}
