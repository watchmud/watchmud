package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// handleSell trades what the player carries for coins, gone for good: a shop
// sells its own stock, not what it bought. Like drop, it won't take what's
// worn, and "sell all.pelt" passes over that and anything worthless without
// a word.
func (w *World) handleSell(msg *gameserver.HandlerParameter, cmd command.Sell) {
	if _, ok := w.shopHere(msg); !ok {
		return
	}
	target, ok := tradeTarget(msg, cmd.Target)
	if !ok {
		return
	}
	found := targetsIn(target, msg.Player.Inventory().All())
	if len(found) == 0 {
		msg.Fail(event.TargetNotFound)
		return
	}
	sold := 0
	var refused event.ResultCode
	for _, inst := range found {
		if msg.Player.Equipment().ItemEquipped(inst) {
			refused = event.TargetInUse
			continue
		}
		// what's in a bag would go with it, and the shop doesn't pay for that
		if inst.Contents != nil && inst.Contents.Len() > 0 {
			refused = event.NotEmpty
			continue
		}
		pay := w.sellPrice(inst)
		if pay == 0 {
			refused = event.Worthless
			continue
		}
		if err := msg.Player.Inventory().Remove(inst); err != nil {
			log.Error().Err(err).Str("player", msg.Player.Name()).Msg("sell")
			msg.Fail(event.Unknown)
			return
		}
		msg.Player.AddCoins(pay)
		msg.Player.Send(event.Sold{Item: inst.Definition.ShortDescription, Coins: pay})
		sold++
	}
	// naming one thing that couldn't be sold says why; "all" says nothing
	// unless nothing at all could be
	if sold == 0 {
		msg.Fail(refused)
	}
}
