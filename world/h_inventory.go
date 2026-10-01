package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

func (w *World) handleInventory(msg *gameserver.HandlerParameter, cmd command.Inventory) {
	var items []event.InventoryItem
	for instPtr := range msg.Player.Inventory().All() {
		if !msg.Player.Equipment().ItemEquipped(instPtr) {
			items = append(items, event.InventoryItem{
				Id:               instPtr.Id.String(),
				ShortDescription: instPtr.Definition.ShortDescription,
				Category:         instPtr.Definition.ObjectCategory,
			})
		}
	}
	msg.Player.Send(event.Inventory{Items: items, Coins: msg.Player.Coins()})
}
