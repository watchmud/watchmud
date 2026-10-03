package world

import (
	"fmt"

	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// cast is one use of an ability, already checked and paid for.
type cast struct {
	caster  *player.Player
	target  *player.Player // for self and friend, foe will add a mob
	ability *rules.Ability
	power   int // of the item granting it
}

// An effect is what an ability does. Content says how much; this says what.
// It can't refuse: by the time it runs the cast is paid for.
type effect func(w *World, c cast)

// effects is every ability the engine knows how to do, by rules.Ability id.
var effects = map[string]effect{
	"heal": healEffect,
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
