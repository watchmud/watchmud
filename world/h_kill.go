package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

func (w *World) handleKill(msg *gameserver.HandlerParameter, cmd command.Kill) {
	// TODO make different kill command for killing a player vs
	// killing a mob. for now this will just be killing mobs.

	// if you're already in a fight, you can't start a new fight
	if w.fightLedger.IsFighting(msg.Player) {
		msg.Fail(event.AlreadyFighting)
		return
	}

	// figure out if the target of your fight is valid
	//	are they in the room (still)
	room := w.playerRoom(msg.Player)
	mobileInstance, exists := room.FindMobile(cmd.Target)
	if !exists {
		msg.Fail(event.TargetNotFound)
		return
	}

	//  does this room allow fighting..
	if room.Flag(spaces.RoomFlagNoFight) {
		msg.Fail(event.NoFightRoom)
		return
	}

	//	are they something you are allowed to fight (no_fight, other flags... objects...)
	if mobileInstance.Definition.HasFlag(rules.MobileFlagPlayerCantFight) {
		msg.Fail(event.NoFight)
		return
	}

	// begin a fight with that target (or join an existing fight if there's
	// already one going on with that target)

	// "Ok." first, so a scripted mob's opener answers it rather than coming
	// before it. The ledger can't refuse here -- IsFighting was checked above
	// -- so "Ok." followed by a failure doesn't happen in practice.
	msg.Player.Send(event.Attacking{Target: mobileInstance.Name()})
	if err := w.startFight(msg.Player, mobileInstance); err != nil {
		log.Error().Str("playerName", msg.Player.Name()).Str("target", mobileInstance.Name()).Err(err).Msg("kill: couldn't start the fight")
		msg.Fail(event.InternalError)
	}
}
