package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
)

// handleWear puts something from the pack on, in the one slot it goes in.
func (w *World) handleWear(msg *gameserver.HandlerParameter, cmd command.Wear) {
	if cmd.Target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	target, err := parseTarget(cmd.Target)
	if err != nil {
		msg.Fail(event.ParseError)
		return
	}
	item, code := pickToWear(msg.Player, target)
	if code != "" {
		msg.Fail(code)
		return
	}
	msg.Player.Equipment().Equip(item.Definition.EquipmentSlot, item)
	msg.Player.Send(event.Worn{Item: item.Definition.ShortDescription})
}

// pickToWear is what "wear <target>" means. Numbered -- 2.leather -- it's
// that one, counted through the pack as every command counts. Otherwise it's
// the first match that can go on now: not already worn, and with its slot
// free. Several things answer to one word -- caps and boots both to
// "leather" -- and refusing over a second cap while the boots sit unworn
// behind it isn't what anyone typing "wear leather" meant.
//
// With nothing that can go on, the refusal is about the first match that
// isn't already worn: not wearable at all, or its slot taken.
func pickToWear(p *player.Player, target Target) (*object.Instance, event.ResultCode) {
	matches := p.Inventory().FindAll(target.Name)
	if target.Identifier > 0 {
		if target.Identifier > len(matches) {
			return nil, event.TargetNotFound
		}
		matches = matches[target.Identifier-1 : target.Identifier]
	}
	if len(matches) == 0 {
		return nil, event.TargetNotFound
	}
	for _, m := range matches {
		if wearRefusal(p, m) == "" {
			return m, ""
		}
	}
	// Nothing can go on: explain the first one that isn't on already -- a
	// second cap is blocked by the first, which is the news -- or, if they
	// all are, that.
	for _, m := range matches {
		if !p.Equipment().ItemEquipped(m) {
			return nil, wearRefusal(p, m)
		}
	}
	return nil, event.InUse
}

// wearRefusal is why item can't go on right now, or "" if it can.
func wearRefusal(p *player.Player, item *object.Instance) event.ResultCode {
	switch {
	case !item.Definition.Wearable():
		return event.CantWearThat
	case p.Equipment().ItemEquipped(item):
		return event.InUse
	case p.Equipment().Equipped(item.Definition.EquipmentSlot):
		return event.LocationInUse
	}
	return ""
}
