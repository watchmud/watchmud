package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
)

// handleGive is "give <item> to <player>": put run towards another player in
// the room instead of a container. The target is get's grammar, coins
// included; what's worn stays on, as drop and put leave it. A bag goes with
// whatever is in it.
func (w *World) handleGive(msg *gameserver.HandlerParameter, cmd command.Give) {
	if cmd.Target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	if cmd.To == "" {
		msg.Fail(event.NoRecipient)
		return
	}
	room := w.playerRoom(msg.Player)
	to, found := room.FindPlayer(cmd.To)
	if !found {
		msg.Fail(event.ToPlayerNotFound)
		return
	}
	if to == msg.Player {
		msg.Fail(event.GiveSelf)
		return
	}
	target, err := parseTarget(cmd.Target)
	if err != nil {
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("target", cmd.Target).Msg("give: can't parse target")
		msg.Fail(event.ParseError)
		return
	}

	if isCoins(target.Name) {
		w.giveCoins(msg, to, target.Quantity)
		return
	}

	items := targetsIn(target, msg.Player.Inventory().All())
	if len(items) == 0 {
		msg.Fail(event.TargetNotFound)
		return
	}
	given := 0
	for _, item := range items {
		if msg.Player.Equipment().ItemEquipped(item) {
			if !target.All {
				msg.Fail(event.TargetInUse)
				return
			}
			continue
		}
		if err := object.Move(item, msg.Player.Inventory(), to.Inventory()); err != nil {
			log.Error().Err(err).Str("player", msg.Player.Name()).Str("to", to.Name()).Msg("give")
			msg.Fail(event.InternalError)
			return
		}
		room.Send(event.Gave{Actor: msg.Player.Name(), Recipient: to.Name(), Item: item.Definition.ShortDescription})
		given++
	}
	if given == 0 {
		msg.Fail(event.TargetInUse)
	}
}

// giveCoins is "give 20 coins to bob". Unlike put, a number is required:
// "give coins to bob" handing over every coin is too easy a mistake.
func (w *World) giveCoins(msg *gameserver.HandlerParameter, to *player.Player, n int) {
	if n == 0 {
		msg.Fail(event.NoValue)
		return
	}
	if !msg.Player.Spend(n) {
		msg.Fail(event.NotEnoughCoins)
		return
	}
	to.AddCoins(n)
	w.playerRoom(msg.Player).Send(event.Gave{Actor: msg.Player.Name(), Recipient: to.Name(), Item: coinsText(n)})
}
