package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/gameserver"
)

func (w *World) handleLook(msg *gameserver.HandlerParameter, cmd command.Look) {
	if cmd.In {
		w.lookIn(msg, cmd.Target)
		return
	}
	if cmd.Target != "" {
		w.lookAt(msg, cmd.Target)
		return
	}
	playerRoom := w.playerRoom(msg.Player)
	msg.Player.Send(playerRoom.DescriptionExcept(msg.Player))
}
