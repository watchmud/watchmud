package world

import (
	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/script"
)

// startFight begins a fight between attacker and defender, then gives each
// mob in it that wasn't already fighting its on_fight_start. Everything that
// starts a fight -- kill and aggro -- comes through here, so no opener is
// missed. Joining a fight a mob is already in isn't a start for that mob, and
// neither is the ledger retargeting it after a death.
func (w *World) startFight(attacker, defender combat.Combatant) error {
	attackerWas, defenderWas := w.fightLedger.InFight(attacker), w.fightLedger.InFight(defender)
	if err := w.fightLedger.Fight(attacker, defender); err != nil {
		return err
	}
	if mob, ok := attacker.(*mobile.Instance); ok && !attackerWas {
		w.scripts.FightStart(mob, foeOf(defender))
	}
	if mob, ok := defender.(*mobile.Instance); ok && !defenderWas {
		w.scripts.FightStart(mob, foeOf(attacker))
	}
	return nil
}

// foeOf is a combatant as a script sees them: a name, and whether they're a
// player.
func foeOf(c combat.Combatant) script.Foe {
	_, isPlayer := c.(*player.Player)
	return script.Foe{Name: c.Name(), IsPlayer: isPlayer}
}

// mobSays is a script's me:say. The mob's room hears it the way it hears a
// player's say.
func (w *World) mobSays(mob *mobile.Instance, text string) {
	w.mobileRoom(mob).Send(event.Said{Speaker: mob.Name(), Value: text})
}

// liveRoller rolls through whatever w.roller is when a script rolls, not what
// it was when the world was built: tests load their own dice after
// NewTestWorld, and a script has to roll those.
type liveRoller struct{ w *World }

func (r liveRoller) IntN(n int) (int, error) { return r.w.roller.IntN(n) }
