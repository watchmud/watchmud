package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// handleColor saves the player's choice of ANSI color. The connection does the
// coloring; the world only remembers which way the player wants it, so it
// survives a logout.
func (w *World) handleColor(msg *gameserver.HandlerParameter, cmd command.Color) {
	on := !msg.Player.Color()
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
	msg.Player.SetColor(on)
	msg.Player.Send(event.Color{On: on, Changed: true})
}

// handleScreenReader saves whether the player wants text for a screen
// reader. Like color, the connection does the rendering and the world only
// remembers the choice.
func (w *World) handleScreenReader(msg *gameserver.HandlerParameter, cmd command.ScreenReader) {
	on := !msg.Player.ScreenReader()
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
	msg.Player.SetScreenReader(on)
	msg.Player.Send(event.ScreenReader{On: on, Changed: true})
}
