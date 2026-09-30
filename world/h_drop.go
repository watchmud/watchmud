package world

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

func (w *World) handleDrop(msg *gameserver.HandlerParameter, cmd command.Drop) {
	if cmd.Target == "" {
		msg.Fail(event.NoTarget)
		return
	}

	target, err := parseTarget(cmd.Target)
	if err != nil {
		// the parse error is for us, not for the player: it's things like
		// TOO_MANY_DOTS and strconv's complaint about "x.knife".
		log.Debug().Err(err).Str("player", msg.Player.Name()).Str("target", cmd.Target).Msg("drop: can't parse target")
		msg.Fail(event.ParseError)
		return
	}

	// TODO handle "coins" (target.Quantity)

	room := w.playerRoom(msg.Player)
	objectsToDrop := targetsIn(target, msg.Player.Inventory().All())
	if len(objectsToDrop) == 0 {
		msg.Fail(event.TargetNotFound)
		return
	}

	dropped := 0
	for _, objectToDrop := range objectsToDrop {
		// is the object cursed?
		// TODO cursed

		// is the object being held or otherwise in use? naming one thing you
		// have on is worth saying so; "drop all" while wearing armor is not.
		if msg.Player.Equipment().ItemEquipped(objectToDrop) {
			if !target.All {
				msg.Fail(event.TargetInUse)
				return
			}
			continue
		}

		if err := object.Move(objectToDrop, msg.Player.Inventory(), room.Inventory); err != nil {
			log.Error().Err(err).Str("player", msg.Player.Name()).Str("room", room.Location().String()).Msg("drop")
			msg.Fail(event.AddToRoomError)
			return
		}
		// on the floor now, so on the clock: see rules.DroppedDecay
		objectToDrop.DecaysAt = time.Now().Add(rules.DroppedDecay)
		// one event, both audiences: the renderer says "Dropped." to the actor
		// and "bob drops a knife." to everyone else.
		room.Send(event.Dropped{
			Actor: msg.Player.Name(),
			Item:  objectToDrop.Definition.ShortDescription, // rendered to clients, so use "a knife"
		})
		dropped++
	}

	// everything named was something you have on
	if dropped == 0 {
		msg.Fail(event.TargetInUse)
	}
}
