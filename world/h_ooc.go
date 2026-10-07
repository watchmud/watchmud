package world

import (
	"strings"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// handleOOC is the out-of-character channel: "ooc <words>" goes to everyone
// on it, wherever they are; "ooc off" leaves it and "ooc on" rejoins, a choice
// kept on the record. Someone off the channel can't speak on it either: a
// question they couldn't hear the answer to.
func (w *World) handleOOC(msg *gameserver.HandlerParameter, cmd command.OOC) {
	switch strings.ToLower(cmd.Value) {
	case "":
		msg.Fail(event.NoValue)
		return
	case "on", "off":
		on := strings.EqualFold(cmd.Value, "on")
		msg.Player.SetOOC(on)
		msg.Player.Send(event.OOCSet{On: on})
		return
	}
	if !msg.Player.OOC() {
		msg.Fail(event.OffChannel)
		return
	}
	said := event.OOCSaid{Speaker: msg.Player.Name(), Value: cmd.Value}
	for p := range w.playerList.All() {
		if p.OOC() {
			p.Send(said)
		}
	}
}
