package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/spaces"
)

func (w *World) handleRestore(msg *gameserver.HandlerParameter, cmd command.Restore) {
	targetRoom := w.playerRoom(msg.Player)

	logWizCommand(msg.Player, "restore", "Player %s is attempting to restore %s",
		msg.Player.Name(), cmd.Target)

	if cmd.Target == "" {
		restorePlayer(msg.Player, targetRoom)
	} else if p, playerFound := targetRoom.FindPlayer(cmd.Target); playerFound {
		restorePlayer(p, targetRoom)
	} else if m, mobFound := targetRoom.FindMobile(cmd.Target); mobFound {
		restoreMob(m, targetRoom)
	} else {
		msg.Fail(event.TargetNotFound)
	}

}

func restorePlayer(p *player.Player, r *spaces.Room) {
	p.RestoreMaxHealth()
	p.RestoreMaxMana()
	r.Notify(event.Restored{
		IsPlayer: true,
		Target:   p.Name(),
	})
}

func restoreMob(m *mobile.Instance, r *spaces.Room) {
	m.RestoreMaxHealth()
	// Mobs don't have mana ... yet
	r.Notify(event.Restored{
		IsPlayer: false,
		Target:   m.Name(),
	})
}
