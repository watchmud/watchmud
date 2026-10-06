package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
)

// handlePut is "put <item> in <container>": get-from run backwards, into an
// open container on the floor. The target is get's grammar -- "all",
// "all.pelt", "2.knife", "20 coins" -- and what's worn stays on, as drop
// leaves it: naming one worn thing says so, "all" passes over them. Nothing
// put in a chest decays: it's being kept, not left lying.
func (w *World) handlePut(msg *gameserver.HandlerParameter, cmd command.Put) {
	if cmd.Target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	if cmd.Into == "" {
		msg.Fail(event.NoContainer)
		return
	}
	container, ok := w.findContainer(msg, cmd.Into)
	if !ok {
		return
	}
	target, err := parseTarget(cmd.Target)
	if err != nil {
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("target", cmd.Target).Msg("put: can't parse target")
		msg.Fail(event.ParseError)
		return
	}
	room := w.playerRoom(msg.Player)
	into := container.Definition.ShortDescription

	if isCoins(target.Name) {
		n := target.Quantity
		if n == 0 {
			n = msg.Player.Coins()
		}
		if n == 0 || !msg.Player.Spend(n) {
			msg.Fail(event.NotEnoughCoins)
			return
		}
		container.Coins += n
		room.Send(event.Put{Actor: msg.Player.Name(), Item: coinsText(n), Into: into})
		return
	}

	items := targetsIn(target, msg.Player.Inventory().All())
	if len(items) == 0 {
		msg.Fail(event.TargetNotFound)
		return
	}
	put := 0
	for _, item := range items {
		if msg.Player.Equipment().ItemEquipped(item) {
			if !target.All {
				msg.Fail(event.TargetInUse)
				return
			}
			continue
		}
		if err := object.Move(item, msg.Player.Inventory(), container.Contents); err != nil {
			log.Error().Err(err).Str("player", msg.Player.Name()).Str("room", room.Location().String()).Msg("put")
			msg.Fail(event.AddToRoomError)
			return
		}
		room.Send(event.Put{Actor: msg.Player.Name(), Item: item.Definition.ShortDescription, Into: into})
		put++
	}
	if put == 0 {
		msg.Fail(event.TargetInUse)
	}
}
