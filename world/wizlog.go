package world

import (
	"github.com/watchmud/watchmud/player"
)

func logWizCommand(p *player.Player, command string, msg string, args ...any) {
	p.Log().Warn().
		Str("commandType", "wiz").
		Str("command", command).
		Msgf(msg, args...)
}
