package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// Tell everybody in the game something.
func (w *World) handleTellAll(msg *gameserver.HandlerParameter, cmd command.TellAll) {
	if cmd.Value == "" {
		msg.Fail(event.NoValue)
		return
	}
	shouted := event.Shouted{
		Speaker: msg.Player.Name(),
		Value:   cmd.Value,
	}
	if !msg.Player.Shouts() {
		msg.Fail(event.ShoutsOff)
		return
	}
	for p := range w.playerList.All() {
		if p.Shouts() {
			p.Send(shouted)
		}
	}
}
