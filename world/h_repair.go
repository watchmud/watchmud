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
// is what "broken, not destroyed" was for. Only at a smithy, and free: there
// is nothing to pay with yet, so the price is the walk back to town.
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

	repaired := 0
	for _, inst := range found {
		if !needsRepair(inst) {
			continue
		}
		inst.Repair()
		msg.Player.Send(event.Repaired{Item: inst.Definition.ShortDescription})
		repaired++
	}
	if repaired == 0 {
		msg.Fail(event.NotDamaged)
	}
}

// needsRepair is gear that wears out and has. Gear that never wears out is
// never damaged, so "repair all" passes over it without a word.
func needsRepair(inst *object.Instance) bool {
	return inst.WearsOut() && inst.Durability < inst.Definition.MaxDurability
}
