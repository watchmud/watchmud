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

// handleJunk is "junk <item>": gone for good, nothing back -- only the space
// it took. What's worn stays on, and a bag with anything in it waits until
// it's emptied: junking what's inside by accident is a loss no one meant.
func (w *World) handleJunk(msg *gameserver.HandlerParameter, cmd command.Junk) {
	w.getRidOf(msg, cmd.Target, true, func(inst *object.Instance) bool {
		if err := msg.Player.Inventory().Remove(inst); err != nil {
			log.Error().Err(err).Str("player", msg.Player.Name()).Msg("junk")
			msg.Fail(event.Unknown)
			return false
		}
		w.playerRoom(msg.Player).Send(event.Junked{Actor: msg.Player.Name(), Item: inst.Definition.ShortDescription})
		return true
	})
}

// handleDonate is "donate <item>", from anywhere: it goes to the donation
// room's floor, where it lies like anything dropped there, for whoever needs
// it. A bag goes with whatever is in it. What's worn stays on.
func (w *World) handleDonate(msg *gameserver.HandlerParameter, cmd command.Donate) {
	d := w.content.Settings.Donation
	donation, found := w.findRoomById(d.ZoneId, d.RoomId)
	if !found {
		msg.Fail(event.NoDonationRoom)
		return
	}
	w.getRidOf(msg, cmd.Target, false, func(inst *object.Instance) bool {
		if err := object.Move(inst, msg.Player.Inventory(), donation.Inventory); err != nil {
			log.Error().Err(err).Str("player", msg.Player.Name()).Msg("donate")
			msg.Fail(event.Unknown)
			return false
		}
		inst.DecaysAt = time.Now().Add(rules.DroppedDecay)
		w.playerRoom(msg.Player).Send(event.Donated{Actor: msg.Player.Name(), Item: inst.Definition.ShortDescription})
		if w.playerRoom(msg.Player) != donation {
			donation.Send(event.Appeared{Item: inst.Definition.ShortDescription})
		}
		return true
	})
}

// getRidOf is junk and donate's shared part: the target, in get's grammar,
// among what the player carries; worn things passed over (and said so when
// named alone); a bag with things in it refused when emptyBags; and each of
// the rest handed to away, which stops everything by answering false.
func (w *World) getRidOf(msg *gameserver.HandlerParameter, raw string, emptyBags bool, away func(*object.Instance) bool) {
	if raw == "" {
		msg.Fail(event.NoTarget)
		return
	}
	target, err := parseTarget(raw)
	if err != nil {
		msg.Fail(event.ParseError)
		return
	}
	if isCoins(target.Name) {
		msg.Fail(event.CoinsInPurse)
		return
	}
	found := targetsIn(target, msg.Player.Inventory().All())
	if len(found) == 0 {
		msg.Fail(event.TargetNotFound)
		return
	}
	done := 0
	refused := event.TargetInUse
	for _, inst := range found {
		if msg.Player.Equipment().ItemEquipped(inst) {
			refused = event.TargetInUse
			continue
		}
		if emptyBags && inst.Contents != nil && inst.Contents.Len() > 0 {
			refused = event.NotEmpty
			continue
		}
		if !away(inst) {
			return
		}
		done++
	}
	if done == 0 {
		msg.Fail(refused)
	}
}
