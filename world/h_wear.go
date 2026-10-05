package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
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
	item, code := pickToWear(msg.Player, target, rules.SlotNone)
	if code != "" {
		msg.Fail(code)
		return
	}
	msg.Player.Equipment().Equip(item.Definition.EquipmentSlot, item)
	msg.Player.Send(event.Worn{Item: item.Definition.ShortDescription})
}

// pickToWear is what "wear <target>" or "wield <target>" means. slot is
// where it has to go -- wield's hand -- or rules.SlotNone for wherever the
// item goes. Numbered -- 2.leather -- it's that one, counted through the pack
// as every command counts. Otherwise it's the first match that can go on now:
// wearable there, not already worn, and with its slot free. Several things
// answer to one word -- caps and boots both to "leather" -- and refusing over
// a second cap while the boots sit unworn behind it isn't what anyone typing
// "wear leather" meant.
//
// With nothing that can go on, the refusal explains the likeliest thing they
// meant: the first match that would fit if its slot were free, else the
// first not already worn, else that it's all on already.
func pickToWear(p *player.Player, target Target, slot rules.EquipmentSlot) (*object.Instance, event.ResultCode) {
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
		if wearRefusal(p, m, slot) == "" {
			return m, ""
		}
	}
	worn := p.Equipment().ItemEquipped
	for _, m := range matches {
		if fits(m, slot) && !worn(m) {
			return nil, wearRefusal(p, m, slot)
		}
	}
	for _, m := range matches {
		if !worn(m) {
			return nil, wearRefusal(p, m, slot)
		}
	}
	return nil, event.InUse
}

// fits is whether item goes in slot at all: rules.SlotNone means its own.
func fits(item *object.Instance, slot rules.EquipmentSlot) bool {
	return item.Definition.Wearable() && (slot == rules.SlotNone || item.Definition.EquipmentSlot == slot)
}

// wearRefusal is why item can't go on in slot right now, or "" if it can.
func wearRefusal(p *player.Player, item *object.Instance, slot rules.EquipmentSlot) event.ResultCode {
	switch {
	case !item.Definition.Wearable():
		return event.CantWearThat
	case !fits(item, slot):
		return event.CantWearThere
	case p.Equipment().ItemEquipped(item):
		return event.InUse
	case p.Equipment().Equipped(item.Definition.EquipmentSlot):
		return event.LocationInUse
	}
	return ""
}
