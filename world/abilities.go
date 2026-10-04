package world

import (
	"fmt"

	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// cast is one use of an ability, already checked and paid for.
type cast struct {
	caster  *player.Player
	target  *player.Player   // for self and friend
	foe     *mobile.Instance // for foe
	ability *rules.Ability
	power   int // of the item granting it
}

// An effect is what an ability does. Content says how much; this says what.
// It can't refuse: by the time it runs the cast is paid for.
type effect func(w *World, c cast)

// effects is every ability the engine knows how to do, by rules.Ability id.
var effects = map[string]effect{
	"heal":    healEffect,
	"smite":   smiteEffect,
	"provoke": provokeEffect,
}

// checkEffects refuses a catalog naming an ability the engine cant do -
// the other half of the loader refusing an object granting one the catalog
// doesn't define.
func checkEffects(cat *rules.Catalog) error {
	for _, a := range cat.AbilityList() {
		if _, ok := effects[a.Id]; !ok {
			return fmt.Errorf("ability %q has no effect in world/abilities.go", a.Id)
		}
	}
	return nil
}

func healEffect(w *World, c cast) {
	before := c.target.CurrentHealth()
	c.target.RestoreHealth(c.ability.Amount.For(c.power))
	w.playerRoom(c.caster).Send(event.Healed{
		Actor:  c.caster.Name(),
		Target: c.target.Name(),
		Amount: c.target.CurrentHealth() - before,
	})
}

// smiteEffect is a blow that always lands -- mana and the cooldown are its
// cost -- harder for a stronger weapon. In the order a melee blow goes: the
// hit, the wear, then the death or the fight it starts.
func smiteEffect(w *World, c cast) {
	room := w.playerRoom(c.caster)
	damage := c.ability.Amount.For(c.power)
	dead := c.foe.TakeMeleeDamage(damage)
	room.Notify(event.Smote{Actor: c.caster.Name(), Target: c.foe.Name(), Damage: damage})
	w.wearFromBlow(c.caster, c.foe, room)
	if dead {
		w.combatantDied(c.foe, room)
		return
	}
	// The ledger won't start a fight for someone already in one, so: not
	// fighting, it opens one both ways, like kill. Already fighting, only the
	// mob turns on you, and your swings stay on whoever you were fighting.
	var err error
	switch {
	case !w.fightLedger.IsFighting(c.caster):
		err = w.startFight(c.caster, c.foe)
	case !w.fightLedger.IsFighting(c.foe):
		err = w.startFight(c.foe, c.caster)
	}
	if err != nil {
		c.caster.Log().Error().Err(err).Msg("smite: starting the fight")
	}
}

// provokeEffect turns the mob on the caster: the one thing that overrides
// "whoever engaged first", so a tank can take a mob back. Like smite, a caster
// who isn't fighting opens a fight both ways; one who is keeps swinging at
// whoever they were.
func provokeEffect(w *World, c cast) {
	room := w.playerRoom(c.caster)
	if fight := w.fightLedger.GetFight(c.foe); fight != nil && fight.Fightee == c.caster {
		room.Send(event.Provoked{Actor: c.caster.Name(), Target: c.foe.Name(), Already: true})
		return
	}
	room.Send(event.Provoked{Actor: c.caster.Name(), Target: c.foe.Name()})
	var err error
	switch {
	case !w.fightLedger.IsFighting(c.caster):
		err = w.startFight(c.caster, c.foe)
	case !w.fightLedger.IsFighting(c.foe):
		err = w.startFight(c.foe, c.caster)
	}
	if err != nil {
		c.caster.Log().Error().Err(err).Msg("provoke: starting the fight")
	}
	if fight := w.fightLedger.GetFight(c.foe); fight == nil || fight.Fightee != c.caster {
		w.fightLedger.Turn(c.foe, c.caster)
	}
}
