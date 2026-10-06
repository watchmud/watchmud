package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

func (w *World) handleExits(msg *gameserver.HandlerParameter, cmd command.Exits) {
	r := w.playerRoom(msg.Player)
	exits := []event.Exit{}
	for _, rexit := range r.Exits(false) {
		exits = append(exits, event.Exit{
			Direction: rexit.Direction,
			RoomName:  rexit.Room.Name,
			Closed:    !rexit.Passable(),
		})
	}
	msg.Player.Send(event.Exits{Exits: exits})
}
