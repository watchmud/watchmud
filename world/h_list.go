package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

func (w *World) handleList(msg *gameserver.HandlerParameter, cmd command.List) {
	shop, ok := w.shopHere(msg)
	if !ok {
		return
	}
	econ := w.content.Catalog.Economy
	var items []event.ShopEntry
	for _, s := range shop.Stock {
		items = append(items, event.ShopEntry{
			Item:  s.Object.ShortDescription,
			Power: s.Power,
			Price: econ.Price(s.Object.ObjectCategory, s.Power),
		})
	}
	msg.Player.Send(event.ShopList{Items: items})
}
