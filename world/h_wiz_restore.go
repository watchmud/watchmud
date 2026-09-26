package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

func (w *World) handleRestore(msg *gameserver.HandlerParameter, cmd command.Restore) {
	targetRoom := w.playerRoom(msg.Player)

	logWizCommand(msg.Player, "restore", "Player %s is attempting to restore %s",
		msg.Player.Name(), cmd.Target)

	// find a matching player
	if targetPlayer, found := targetRoom.FindPlayer(cmd.Target); found {
		targetPlayer.RestoreMaxHealth()
		targetRoom.Notify(event.Restored{
			IsPlayer: true,
			Target:   targetPlayer.Name(),
		})
		return
	}

	// find a matching mob
	if targetMob, found := targetRoom.FindMobile(cmd.Target); found {
		targetMob.RestoreMaxHealth()
		targetRoom.Notify(event.Restored{
			IsPlayer: false,
			Target:   targetMob.Name(),
		})
		return
	}
	msg.Fail(event.TargetNotFound)
}
