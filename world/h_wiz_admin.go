package world

import (
	"slices"
	"strings"

	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/spaces"
)

// handleGoto takes a wizard to a room by "zone/room", or to whoever -- a
// player anywhere, then a mob anywhere -- that name finds.
func (w *World) handleGoto(msg *gameserver.HandlerParameter, cmd command.Goto) {
	logWizCommand(msg.Player, "goto", "%s goes to %s", msg.Player.Name(), cmd.Target)
	dest := w.placeNamed(cmd.Target)
	if dest == nil {
		msg.Fail(event.TargetNotFound)
		return
	}
	// a wizard leaving a fight leaves it, as anyone transferred does: a
	// fight across rooms never swings and never ends
	w.fightLedger.EndAllFightsWith(msg.Player.Id())
	w.movePlayerMagically(msg.Player, dest)
	msg.Player.Send(dest.DescriptionExcept(msg.Player))
}

// placeNamed is the room a wizard means: "zone/room", a player, or a mob.
func (w *World) placeNamed(target string) *spaces.Room {
	if zoneId, roomId, ok := strings.Cut(target, "/"); ok {
		if r, found := w.findRoomById(zoneId, roomId); found {
			return r
		}
		return nil
	}
	if p := w.findPlayerByName(target); p != nil {
		return w.playerRoom(p)
	}
	for _, mob := range w.occupancy.Mobiles() {
		if mob.Matches(target) {
			return w.mobileRoom(mob)
		}
	}
	return nil
}

// handleTransfer brings a player to the wizard, out of any fight first.
func (w *World) handleTransfer(msg *gameserver.HandlerParameter, cmd command.Transfer) {
	logWizCommand(msg.Player, "transfer", "%s transfers %s", msg.Player.Name(), cmd.Target)
	p := w.findPlayerByName(cmd.Target)
	if p == nil {
		msg.Fail(event.ToPlayerNotFound)
		return
	}
	dest := w.playerRoom(msg.Player)
	w.fightLedger.EndAllFightsWith(p.Id())
	w.movePlayerMagically(p, dest)
	p.Send(dest.DescriptionExcept(p))
}

// handlePurge clears the room of every mob and everything on the floor, or,
// named, one mob or the things that name finds. Players are never purged.
func (w *World) handlePurge(msg *gameserver.HandlerParameter, cmd command.Purge) {
	logWizCommand(msg.Player, "purge", "%s purges %q", msg.Player.Name(), cmd.Target)
	room := w.playerRoom(msg.Player)
	var purged event.Purged
	if cmd.Target != "" {
		if mob, found := room.FindMobile(cmd.Target); found {
			w.fightLedger.EndAllFightsWith(mob.Id())
			w.RemoveMobile(mob)
			purged.Mobs++
		} else if t, err := parseTarget(cmd.Target); err == nil {
			for _, inst := range targetsIn(t, room.Inventory.All()) {
				if room.Inventory.Remove(inst) == nil {
					purged.Objects++
				}
			}
		}
		if purged.Mobs+purged.Objects == 0 {
			msg.Fail(event.TargetNotFound)
			return
		}
		msg.Player.Send(purged)
		return
	}
	for _, mob := range slices.Clone(room.Mobiles()) {
		w.fightLedger.EndAllFightsWith(mob.Id())
		w.RemoveMobile(mob)
		purged.Mobs++
	}
	for _, inst := range slices.Collect(room.Inventory.All()) {
		if room.Inventory.Remove(inst) == nil {
			purged.Objects++
		}
	}
	msg.Player.Send(purged)
}

// handleZReset resets a zone now, as its timer would: the wizard's own, or
// the one named.
func (w *World) handleZReset(msg *gameserver.HandlerParameter, cmd command.ZReset) {
	logWizCommand(msg.Player, "zreset", "%s resets %q", msg.Player.Name(), cmd.Zone)
	z := w.playerRoom(msg.Player).Zone
	if cmd.Zone != "" {
		z = w.content.Zones[cmd.Zone]
	}
	if z == nil {
		msg.Fail(event.UnknownZone)
		return
	}
	for _, err := range z.Reset(w.occupancy) {
		msg.Player.Log().Warn().Err(err).Str("zone", z.Id).Msg("zreset")
	}
	msg.Player.Send(event.ZoneWasReset{Zone: z.Name})
}

// handleEcho puts text in front of the room, or everyone: an announcement,
// "the server restarts in five minutes".
func (w *World) handleEcho(msg *gameserver.HandlerParameter, cmd command.Echo) {
	if cmd.Text == "" {
		msg.Fail(event.NoValue)
		return
	}
	logWizCommand(msg.Player, cmd.Verb(), "%s: %s", msg.Player.Name(), cmd.Text)
	if cmd.Global {
		for p := range w.playerList.All() {
			p.Send(event.Echoed{Text: cmd.Text})
		}
		return
	}
	w.playerRoom(msg.Player).Send(event.Echoed{Text: cmd.Text})
}

// handleUsers lists everyone playing, where they are, and any moderation.
func (w *World) handleUsers(msg *gameserver.HandlerParameter, cmd command.Users) {
	var list event.UserList
	for p := range w.playerList.All() {
		u := event.User{Name: p.Name(), Wizard: p.IsWizard(), Bot: p.IsBot(), Muted: p.Muted(), Frozen: p.Frozen()}
		if r := w.playerRoom(p); r != nil {
			u.Room = r.Name
			if r.Zone != nil {
				u.Zone = r.Zone.Name
			}
		}
		list.Users = append(list.Users, u)
	}
	msg.Player.Send(list)
}

// handleModerate switches mute or freeze on a player who's playing. It's on
// their record from the next save, so a logout doesn't shake it off. A
// wizard can't be moderated: that's a conversation, not a command.
func (w *World) handleModerate(msg *gameserver.HandlerParameter, cmd command.Moderate) {
	p := w.findPlayerByName(cmd.Target)
	if p == nil {
		msg.Fail(event.ToPlayerNotFound)
		return
	}
	if p.IsWizard() {
		msg.Fail(event.TargetInUse)
		return
	}
	on := !p.Muted()
	if cmd.Freeze {
		on = !p.Frozen()
		p.SetFrozen(on)
	} else {
		p.SetMuted(on)
	}
	logWizCommand(msg.Player, cmd.Verb(), "%s sets %s %s=%v", msg.Player.Name(), p.Name(), cmd.Verb(), on)
	e := event.Moderated{Target: p.Name(), Freeze: cmd.Freeze, On: on}
	msg.Player.Send(e)
	if p != msg.Player {
		p.Send(e)
	}
}
