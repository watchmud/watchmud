package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
)

func (w *World) handleInventory(msg *gameserver.HandlerParameter, cmd command.Inventory) {
	var items []event.InventoryItem
	for instPtr := range msg.Player.Inventory().All() {
		if !msg.Player.Equipment().ItemEquipped(instPtr) {
			items = append(items, event.InventoryItem{
				Id:               instPtr.Id.String(),
				ShortDescription: instPtr.Definition.ShortDescription,
				Category:         instPtr.Definition.ObjectCategory,
				Power:            instPtr.Power,
				Durability:       instPtr.Durability,
				MaxDurability:    instPtr.Definition.MaxDurability,
				Broken:           instPtr.Broken(),
				Bag:              instPtr.Contents != nil,
				Holding:          bagHolding(instPtr),
			})
		}
	}
	msg.Player.Send(event.Inventory{Items: items, Coins: msg.Player.Coins()})
}

func bagHolding(inst *object.Instance) int {
	if inst.Contents == nil {
		return 0
	}
	return inst.Contents.Len()
}
