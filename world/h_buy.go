package world

import (
	"uuid"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
)

// handleBuy makes a new one of something a shop sells, at the shop's power,
// and hands it over for its price. One at a time: "buy all" in a shop that
// never runs out would be a way to empty a purse by accident.
func (w *World) handleBuy(msg *gameserver.HandlerParameter, cmd command.Buy) {
	shop, ok := w.shopHere(msg)
	if !ok {
		return
	}
	target, ok := tradeTarget(msg, cmd.Target)
	if !ok {
		return
	}
	n := max(target.Identifier, 1)
	for _, s := range shop.Stock {
		if !s.Object.Matches(target.Name) {
			continue
		}
		if n--; n > 0 {
			continue
		}
		item := object.NewInstance(uuid.New(), s.Object)
		item.Power = s.Power
		cost := w.price(item)
		if !msg.Player.Spend(cost) {
			msg.Player.Send(event.TooExpensive{Item: s.Object.ShortDescription, Cost: cost, Coins: msg.Player.Coins()})
			return
		}
		if err := msg.Player.Inventory().Add(item); err != nil {
			msg.Player.AddCoins(cost) // a fresh uuid can't collide; if it did, they paid for nothing
			msg.Fail(event.Unknown)
			return
		}
		msg.Player.Send(event.Bought{Item: s.Object.ShortDescription, Cost: cost})
		return
	}
	msg.Fail(event.NotForSale)
}
