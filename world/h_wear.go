package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

func (w *World) handleWear(msg *gameserver.HandlerParameter, cmd command.Wear) {
	objectsToWear := msg.Player.Inventory().FindAll(cmd.Target)

	if len(objectsToWear) == 0 {
		// nothing in inventory with that name
		msg.Fail(event.TargetNotFound)
		return
	}

	// TODO for now only using first object returned
	objectToWear := objectsToWear[0]

	if !objectToWear.Definition.Wearable() {
		msg.Fail(event.CantWearThat)
		return
	}

	// figure out what the wear location is:
	//		was one provided? (for now, instances can only be worn in one place)
	// 		so we ignore the given location
	loc := objectToWear.Definition.EquipmentSlot

	// is something else already in the location?
	if msg.Player.Equipment().Equipped(loc) {
		msg.Fail(event.InUse)
		return
	}

	// otherwise add the item to the location
	msg.Player.Equipment().Equip(loc, objectToWear)
	msg.Player.Send(event.Worn{})
}
