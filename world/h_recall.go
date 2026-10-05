package world

import (
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/gameserver"
)

// handleRecall is the recall ability under its own name, so a player keeps
// typing what they always have. A temple token grants it, a wizard always has
// it, and it's refused mid-fight -- a free, certain escape would make flee,
// which can fail, pointless. See recallEffect and abilities.json.
func (w *World) handleRecall(msg *gameserver.HandlerParameter, _ command.Recall) {
	w.cast(msg, "recall", "")
}
