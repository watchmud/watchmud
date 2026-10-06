package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

func (w *World) handleMove(msg *gameserver.HandlerParameter, cmd command.Move) {
	// is player in a fight?
	if w.fightLedger.InFight(msg.Player) {
		msg.Fail(event.InAFight)
		return
	}

	src := w.playerRoom(msg.Player)
	dir := cmd.Direction

	msg.Player.Log().Trace().Msgf("player wants to move %s", dir.String())

	// can player go in that direction?
	dest := src.DestinationRoom(dir)
	if dest == nil {
		msg.Fail(event.CantGoThatWay)
		return
	}
	if !src.Passable(dir) {
		msg.Fail(event.DoorShut)
		return
	}
	w.movePlayer(msg.Player, dir, dest)
	msg.Player.Send(dest.DescriptionExcept(msg.Player))
	w.followersCome(msg.Player, src, dir, dest)
}
