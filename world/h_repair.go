package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// handleRepair makes worn gear as good as new, broken gear included -- that
// is what "broken, not destroyed" was for. Only at a smithy, and for coins:
// rules.Economy.RepairCost, more for valuable gear and more for worn. Each
// piece is paid for as it's mended, so "repair all" with too little mends
// what it can, in order, and says what it couldn't afford.
//
// What's worn is searched along with what's carried, since wearing something
// never takes it out of the inventory.
func (w *World) handleRepair(msg *gameserver.HandlerParameter, cmd command.Repair) {
	if cmd.Target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	if !w.playerRoom(msg.Player).Flag(rules.RoomFlagSmithy) {
		msg.Fail(event.NoSmith)
		return
	}
	target, err := parseTarget(cmd.Target)
	if err != nil {
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("target", cmd.Target).Msg("repair: can't parse target")
		msg.Fail(event.ParseError)
		return
	}

	found := targetsIn(target, msg.Player.Inventory().All())
	if len(found) == 0 {
		msg.Fail(event.TargetNotFound)
		return
	}

	damaged := 0
	for _, inst := range found {
		if !needsRepair(inst) {
			continue
		}
		damaged++
		cost := w.repairCost(inst)
		if !msg.Player.Spend(cost) {
			msg.Player.Send(event.TooExpensive{Item: inst.Definition.ShortDescription, Cost: cost, Coins: msg.Player.Coins()})
			continue
		}
		inst.Repair()
		msg.Player.Send(event.Repaired{Item: inst.Definition.ShortDescription, Cost: cost})
	}
	if damaged == 0 {
		msg.Fail(event.NotDamaged)
	}
}

// repairCost is what the smith charges to mend inst from where it is to new.
func (w *World) repairCost(inst *object.Instance) int {
	econ := w.content.Catalog.Economy
	price := econ.Price(inst.Definition.ObjectCategory, inst.Power)
	max := inst.Definition.MaxDurability
	return econ.RepairCost(price, max-inst.Durability, max)
}

// needsRepair is gear that wears out and has. Gear that never wears out is
// never damaged, so "repair all" passes over it without a word.
func needsRepair(inst *object.Instance) bool {
	return inst.WearsOut() && inst.Durability < inst.Definition.MaxDurability
}
