package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

const fleeAttempts = 6

func (w *World) handleFlee(msg *gameserver.HandlerParameter, cmd command.Flee) {
	if !w.fightLedger.IsFighting(msg.Player) {
		// you're not in a fight?!
		msg.Fail(event.NoFight)
		return
	}
	if !w.flee(msg.Player) {
		msg.Fail(event.CantFlee)
	}
}

// flee is a fighting player trying to run: a few tries at a random way out,
// the room told of each, and out of every fight if one is open. It answers
// whether they got away. The flee command and wimpy both come here.
func (w *World) flee(p *player.Player) bool {
	// TODO stunned, disabled, other status effects
	src := w.playerRoom(p)
	for range fleeAttempts {
		src.Send(event.Fleeing{Who: p.Name()})
		// pick a direction at random out of all possible
		i, err := w.roller.IntN(len(rules.AllUsableDirections))
		if err != nil {
			log.Error().Err(err).Msg("flee: failed to generate random direction")
			return false
		}
		dir := rules.AllUsableDirections[i]
		if src.Passable(dir) {
			src.Send(event.Fled{Who: p.Name()})
			w.movePlayer(p, dir, src.DestinationRoom(dir))
			w.fightLedger.EndAllFightsWith(p.Id())
			return true
		}
		src.Send(event.FleeAttemptFailed{Who: p.Name()})
	}
	return false
}
