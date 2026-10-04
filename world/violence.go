package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

// DoViolence walks through all the combat going on and
// makes the combat happen. For each fight, determine if
// it is "time" to do something, and if so determine what to do.
// Update the state and continue.
func (w *World) DoViolence(pulse rules.PulseCount) {

	for _, fight := range w.fightLedger.GetFights() {
		if fight.Fighter.Dead() || fight.Fightee.Dead() {
			continue
		}

		// each fighter should have a speed, like fast medium slow,
		// and then we can take that into account vs the last time
		// that Violence happened - comparing it to PulseCount.
		// I don't want to have the details of pulse count or real-world
		// clocks being part of mob definitions as that will make it a
		// headache to tune these settings.
		if fight.CanDoViolence(w.content.Catalog.MudTime.Violence, pulse) {
			fight.LastPulse = pulse
			// A stunned round is a spent round: no swing, no wear, no script.
			if w.fightLedger.SpendStun(fight.Fighter) {
				if room := w.roomOf(fight.Fighter); room != nil {
					room.Notify(event.Staggered{Name: fight.Fighter.Name()})
				}
				continue
			}
			fightResult, err := combat.AttemptMeleeAttack(w.roller, fight.Fighter, fight.Fightee)
			if err != nil {
				log.Error().Err(err).Msg("failed to attempt melee attack")
				continue
			}
			var isDead = false
			if fightResult.WasHit {
				isDead = fight.Fightee.TakeMeleeDamage(fightResult.Damage)
			}
			// tell everyone what is going on
			room := w.roomOf(fight.Fighter)
			if room != nil {
				room.Notify(event.Struck{
					Attacker: fight.Fighter.Name(),
					Target:   fight.Fightee.Name(),
					Hit:      fightResult.WasHit,
					Damage:   int(fightResult.Damage),
				})
			}

			if fightResult.WasHit {
				// A landed blow costs the gear on both ends of it. After the
				// damage, so a killing blow still wears the armor it went
				// through, and after the room has been told about the blow,
				// so "your tunic gives out" follows the hit that finished it
				// instead of preceding it.
				w.wearFromBlow(fight.Fighter, fight.Fightee, room)
			}

			// A scripted mob gets its say after the room has seen the blow,
			// and not over a body: a killing blow gets no taunt.
			if mob, ok := fight.Fighter.(*mobile.Instance); ok && !isDead && !fight.Fighter.Dead() {
				w.scripts.FightPulse(mob, foeOf(fight.Fightee))
			}

			if isDead {
				w.combatantDied(fight.Fightee, room)
				// TODO award points or other reward
			}
		}
	}
}

// combatantDied cleans up after the one who died and tells the room.
//
// Only the dead one leaves the ledger. This used to end every fight the
// *winner* was in as well, which meant killing one of two attackers quietly
// took you out of the fight with the other one -- combat with more than one
// attacker could never happen. becomeCorpse already ends the dead one's
// fights, in both directions, which is the whole of what should end here.
func (w *World) combatantDied(dead combat.Combatant, room *spaces.Room) {
	w.becomeCorpse(dead)
	if room != nil {
		_, isPlayer := dead.(*player.Player)
		room.Notify(event.Died{
			Target:   dead.Name(),
			IsPlayer: isPlayer,
		})
	}

	// Dying is hard on your kit. After the room has been told, so the toll
	// reads as a consequence of the death rather than as something that
	// happened on the way to it. Mobs have no equipment to lose.
	if p, isPlayer := dead.(*player.Player); isPlayer {
		w.wearFromDeath(p, room)
		w.playerRevives(p)
	}
}

// playerRevives is the rest of a player's death: there is no corpse, the
// player wakes up in the death room with one hit point. Last, so the death
// and its toll are read in the room it happened in.
func (w *World) playerRevives(p *player.Player) {
	p.Revive()
	w.movePlayerMagically(p, w.DeathRoom)
	p.Send(w.DeathRoom.DescriptionExcept(p))

	// This happens on a pulse, not a command, so no save follows it. Without
	// this, a crash before their next command brings them back at full health,
	// in the room they died in, with their gear as it was.
	if err := w.store.Save(w.record(p)); err != nil {
		p.Log().Err(err).Msg("saving player after death")
	}
}
