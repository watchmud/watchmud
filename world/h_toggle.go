package world

import (
	"strings"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// handleToggle lists what a player can switch, or switches one: color, the
// ooc channel, tells, shouts, assist. notell and noshout are its shorthands.
// Color goes through event.Color, as the color command does, so the
// connection hears it too.
func (w *World) handleToggle(msg *gameserver.HandlerParameter, cmd command.Toggle) {
	p := msg.Player
	switch strings.TrimSuffix(strings.ToLower(cmd.Name), "s") {
	case "":
		p.Send(event.Toggles{Color: p.Color(), OOC: p.OOC(), Tells: p.Tells(), Shouts: p.Shouts(),
			Assist: !w.groups.noAssist[p], ScreenReader: p.ScreenReader()})
	case "color", "colour":
		p.SetColor(!p.Color())
		p.Send(event.Color{On: p.Color(), Changed: true})
	case "screenreader":
		p.SetScreenReader(!p.ScreenReader())
		p.Send(event.ScreenReader{On: p.ScreenReader(), Changed: true})
	case "ooc":
		p.SetOOC(!p.OOC())
		p.Send(event.Toggled{Name: "ooc", On: p.OOC()})
	case "tell":
		p.SetTells(!p.Tells())
		p.Send(event.Toggled{Name: "tells", On: p.Tells()})
	case "shout":
		p.SetShouts(!p.Shouts())
		p.Send(event.Toggled{Name: "shouts", On: p.Shouts()})
	case "assist":
		w.handleAssist(msg, command.Assist{})
	default:
		msg.Fail(event.UnknownToggle)
	}
}
