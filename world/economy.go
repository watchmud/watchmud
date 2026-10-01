package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/spaces"
)

// What things are worth, from rules.Economy: one place, so repair, the
// store and value can never disagree about a price.

// price is what a new one of inst would cost.
func (w *World) price(inst *object.Instance) int {
	return w.content.Catalog.Economy.Price(inst.Definition.ObjectCategory, inst.Power)
}

// sellPrice is what a shop pays for inst as it is: the economy's share of its
// price, less for wear. Broken is worth nothing -- it's the smith's business
// first -- and that keeps repair-then-sell from ever paying.
func (w *World) sellPrice(inst *object.Instance) int {
	price := w.price(inst)
	if inst.WearsOut() {
		price = price * inst.Durability / inst.Definition.MaxDurability
	}
	return w.content.Catalog.Economy.SellPrice(price)
}

// shopHere is the shop in the player's room, failing the command with
// NO_SHOP when there isn't one.
func (w *World) shopHere(msg *gameserver.HandlerParameter) (*spaces.Shop, bool) {
	room := w.playerRoom(msg.Player)
	shop, ok := room.Zone.Shops[room.Id]
	if !ok {
		msg.Fail(event.NoShop)
	}
	return shop, ok
}

// tradeTarget parses what a trading command named, failing it when there's
// nothing named or it doesn't parse.
func tradeTarget(msg *gameserver.HandlerParameter, raw string) (Target, bool) {
	if raw == "" {
		msg.Fail(event.NoTarget)
		return Target{}, false
	}
	target, err := parseTarget(raw)
	if err != nil {
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("target", raw).Msg("trade: can't parse target")
		msg.Fail(event.ParseError)
		return Target{}, false
	}
	return target, true
}
