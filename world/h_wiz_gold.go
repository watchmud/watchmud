package world

import (
	"strconv"
	"strings"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
)

// maxGold is the most one gold command conjures: plenty for testing a shop,
// short of a typo that wrecks the numbers.
const maxGold = 1_000_000

// handleGold puts coins in the wizard's own purse -- for trying out shops,
// repair and the rest of what coins are for.
func (w *World) handleGold(msg *gameserver.HandlerParameter, cmd command.Gold) {
	n, err := strconv.Atoi(strings.TrimSpace(cmd.Amount))
	if err != nil || n < 1 || n > maxGold {
		msg.Fail(event.BadRequest)
		return
	}
	msg.Player.AddCoins(n)
	msg.Player.Send(event.GoldGiven{Amount: n, Coins: msg.Player.Coins()})
}
