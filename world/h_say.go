package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/script"
)

func (w *World) handleSay(msg *gameserver.HandlerParameter, cmd command.Say) {
	room := w.playerRoom(msg.Player)
	room.Send(event.Said{
		Speaker: msg.Player.Name(),
		Value:   cmd.Value,
	})
	// then every scripted mob in earshot gets its on_hear. Only a player's
	// say: a mob's me:say goes out without it, so two scripts can't talk
	// each other round in circles.
	speaker := script.Foe{Name: msg.Player.Name(), IsPlayer: true}
	for _, mob := range room.Mobiles() {
		if mob.Definition.Script != "" {
			w.scripts.Hear(mob, speaker, cmd.Value)
		}
	}
}
