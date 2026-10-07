package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

func (w *World) handlePing(msg *gameserver.HandlerParameter, cmd command.Ping) {
	// trace is off unless the log level asks for it
	log.Trace().Str("player", msg.Player.Name()).Msg("ping")
	msg.Player.Send(event.Pong{Target: cmd.Target})
}
