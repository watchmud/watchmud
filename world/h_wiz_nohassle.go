package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// handleNoHassle switches whether aggressive mobs leave the wizard alone. On
// by default (World.Arrive); off is for testing aggro on your own character.
// Only aggro: a mob you attack, or one already fighting you, still fights.
func (w *World) handleNoHassle(msg *gameserver.HandlerParameter, cmd command.NoHassle) {
	on := !msg.Player.NoHassle()
	switch cmd.Setting {
	case "":
	case "on":
		on = true
	case "off":
		on = false
	default:
		msg.Fail(event.BadRequest)
		return
	}
	logWizCommand(msg.Player, "nohassle", "Player %s set nohassle %v", msg.Player.Name(), on)
	msg.Player.SetNoHassle(on)
	msg.Player.Send(event.NoHassle{On: on})
}
