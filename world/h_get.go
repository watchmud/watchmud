package world

import (
	log "github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
)

func (w *World) handleGet(msg *gameserver.HandlerParameter, cmd command.Get) {
	if cmd.Target == "" {
		msg.Fail(event.NoTarget)
		return
	}
	if cmd.From != "" {
		w.getFrom(msg, cmd)
		return
	}

	target, err := parseTarget(cmd.Target)
	if err != nil {
		// the parse error is for us, not for the player: it's things like
		// TOO_MANY_DOTS and strconv's complaint about "x.knife".
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("target", cmd.Target).Msg("get: can't parse target")
		msg.Fail(event.ParseError)
		return
	}

	// TODO handle "coins" (target.Quantity)

	room := w.playerRoom(msg.Player)
	items := targetsIn(target, room.Inventory.All())
	if len(items) == 0 {
		msg.Fail(event.TargetNotFound)
		return
	}

	got := 0
	for _, item := range items {
		if !item.Definition.Takeable() {
			// naming one thing that can't be picked up is worth saying so;
			// "get all" in a room with a fountain in it is not.
			if !target.All {
				msg.Fail(event.TargetNotGettable)
				return
			}
			continue
		}

		// remove from room
		if err := object.Move(item, room.Inventory, msg.Player.Inventory()); err != nil {
			log.Error().Err(err).Str("player", msg.Player.Name()).Str("room", room.Location().String()).Msg("get")
			msg.Fail(event.RemoveFromRoomError)
			return
		}
		room.Send(event.Got{
			Actor: msg.Player.Name(),
			Item:  item.Definition.ShortDescription,
		})
		got++
	}

	// everything named was something you can't pick up
	if got == 0 {
		msg.Fail(event.TargetNotGettable)
	}
}
