package world

import (
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// becomeCorpse if you are dead
func (w *World) becomeCorpse(deadCombatant combat.Combatant) {
	log.Printf("%s is dead!", deadCombatant.Name())

	// if you were fighting, you stop
	w.fightLedger.EndAllFightsWith(deadCombatant.Id())

	switch c := deadCombatant.(type) {
	case *mobile.Instance:
		w.becomeMobileCorpse(c)
		// A player leaves no corpse: combatantDied moves them to the death
		// room instead.
	}
}

// BecomeMobileCorpse turns a mobile instance into a corpse.
// The corpse has the loot from the mobile instance.
func (w *World) becomeMobileCorpse(m *mobile.Instance) {
	// create a corpse for the mobile instance
	// load the corpse with loot
	corpseName := fmt.Sprintf("the corpse of %s", m.Definition.Name)
	d := object.NewDefinition("",
		corpseName,
		"",
		rules.ObjectCategoryCorpse,
		append([]string{"corpse"}, m.Definition.Aliases...),
		corpseName,
		fmt.Sprintf("The corpse of %s is lying here.", m.Definition.Name),
		rules.SlotNone,
		// A corpse is not armor. It used to claim to be cloth, which was
		// worth nothing only because the armor table has no row for the slot
		// it can't be worn in anyway -- two accidents holding hands. Saying
		// none means it stays worth nothing if either of those changes.
		rules.ArmorTypeNone,
		// you loot a corpse, you don't carry it off.
		[]rules.ObjectBehavior{rules.ObjectBehaviorNoTake},
	)

	corpse := object.NewInstance(uuid.New(), d)
	corpse.Contents = object.NewList()
	corpse.DecaysAt = time.Now().Add(rules.CorpseDecay)
	for _, drop := range w.rollLoot(m) {
		if err := corpse.Contents.Add(drop); err != nil {
			log.Error().Err(err).Msgf("becomeMobileCorpse: adding %s to the corpse of %s", drop.Definition.Name, m.Definition.Name)
		}
	}
	// and whatever it had picked up
	for _, taken := range slices.Collect(m.Inventory.All()) {
		if err := object.Move(taken, m.Inventory, corpse.Contents); err != nil {
			log.Error().Err(err).Msgf("becomeMobileCorpse: %s's %s", m.Definition.Name, taken.Definition.Name)
		}
	}
	corpse.Coins = w.rollCoins(m) // after the loot, so its dice come first
	r := w.mobileRoom(m)
	w.RemoveMobile(m)
	if err := r.Inventory.Add(corpse); err != nil {
		log.Error().Msgf("becomeMobileCorpse: could not add corpse %s to room %s, %v", corpse.Definition.Name, r.Name, err)
	}
}

// rollCoins is what a mob was carrying: more the stronger it is. See
// rules.Economy.Coins.
func (w *World) rollCoins(m *mobile.Instance) int {
	econ := w.content.Catalog.Economy
	span := econ.CoinRange(m.Power())
	if span == 0 {
		return 0
	}
	roll, err := w.roller.IntN(span)
	if err != nil {
		log.Error().Err(err).Msgf("rollCoins: %s", m.Definition.Id)
		roll = 0
	}
	return econ.Coins(m.Power(), roll)
}

// rollLoot rolls each line of a mob's loot table on its own -- a d100 against
// the entry's chance -- and makes whatever comes up, at the mob's power plus
// rules.LootPowerBump. At the *mob's* power, never the killer's: out-level a
// boss and his drops stop being worth having, on purpose (LEVELS.md).
//
// A roll that errors is a bug in the dice, not a reason to lose the corpse;
// that entry just doesn't drop.
func (w *World) rollLoot(m *mobile.Instance) []*object.Instance {
	var drops []*object.Instance
	for _, entry := range m.Definition.Loot {
		roll, err := w.roller.IntN(100)
		if err != nil {
			log.Error().Err(err).Msgf("rollLoot: %s", m.Definition.Id)
			continue
		}
		if roll >= entry.Chance {
			continue
		}
		bump, err := w.roller.IntN(100)
		if err != nil {
			log.Error().Err(err).Msgf("rollLoot: %s", m.Definition.Id)
			bump = 100 // no bump
		}
		item := object.NewInstance(uuid.New(), entry.Object)
		item.Power = m.Power() + rules.LootPowerBump(bump)
		drops = append(drops, item)
	}
	return drops
}

// DecayFloors clears away everything on a floor whose time is up -- corpses
// and anything dropped -- based on time.Now().
func (w *World) DecayFloors() {
	w.decayFloors(time.Now())
}

// decayFloors removes whatever on a room's floor has a DecaysAt that has
// passed -- anything still inside goes with it -- and tells the room.
func (w *World) decayFloors(now time.Time) {
	for _, zone := range w.content.Zones {
		for _, room := range zone.Rooms {
			var gone []*object.Instance
			for item := range room.Inventory.All() {
				if !item.DecaysAt.IsZero() && !now.Before(item.DecaysAt) {
					gone = append(gone, item)
				}
			}
			for _, item := range gone {
				if err := room.Inventory.Remove(item); err != nil {
					log.Error().Err(err).Msgf("decayFloors: removing %s from %s", item.Definition.Name, room.Id)
					continue
				}
				room.Send(event.Decayed{Item: item.Definition.ShortDescription})
			}
		}
	}
}
