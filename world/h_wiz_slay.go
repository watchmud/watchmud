package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// handleSlay kills a mob in the room outright, for testing. It goes through
// combatantDied like any killing blow, so whatever a real kill does -- fights
// end, a corpse with the loot and coins rolled, the room seeing it die --
// this does too. Mobs only: a player's name finds nothing.
func (w *World) handleSlay(msg *gameserver.HandlerParameter, cmd command.Slay) {
	if cmd.Target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	room := w.playerRoom(msg.Player)
	mob, found := room.FindMobile(cmd.Target)
	if !found {
		msg.Fail(event.TargetNotFound)
		return
	}
	logWizCommand(msg.Player, "slay", "Player %s slays %s in %s",
		msg.Player.Name(), mob.Definition.Id, room.Location().String())

	mob.CurHealth = 0
	room.Notify(event.Slain{Actor: msg.Player.Name(), Target: mob.Name()})
	w.combatantDied(mob, room)
}
