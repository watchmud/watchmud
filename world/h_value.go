package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// handleValue is what sell would pay, without selling: one thing, the first
// that matches.
func (w *World) handleValue(msg *gameserver.HandlerParameter, cmd command.Value) {
	if _, ok := w.shopHere(msg); !ok {
		return
	}
	target, ok := tradeTarget(msg, cmd.Target)
	if !ok {
		return
	}
	target.All = false
	found := targetsIn(target, msg.Player.Inventory().All())
	if len(found) == 0 {
		msg.Fail(event.TargetNotFound)
		return
	}
	inst := found[0]
	pay := w.sellPrice(inst)
	if pay == 0 {
		msg.Fail(event.Worthless)
		return
	}
	msg.Player.Send(event.Valued{Item: inst.Definition.ShortDescription, Coins: pay})
}
