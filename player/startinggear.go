package player

import (
	"uuid"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// GiveStartingGear creates the kit from content/rules/starting_gear.json and
// puts it on a brand-new character: every item goes into the inventory, and
// the ones marked equip are also worn. Wearing something never takes it out of
// inventory (see handleWear), so this is the same two steps a player would do
// by hand with get and wear.
//
// It is only for a character being created. A returning player's gear comes
// back from their record instead, so calling this on a login would hand them a
// second tunic every time.
//
// Nothing here fails: loader.checkStartingGear already refused to start the
// server on a kit that doesn't add up, so anything still wrong at this point
// is a bug rather than a content edit, and it is not worth turning a
// character's first ever command into an error. Log it and hand them the rest.
func GiveStartingGear(p *Player, gear rules.StartingGear, defs DefinitionSource) {
	for _, item := range gear {
		d, found := defs.ObjectDefinition(item.ZoneId, item.DefinitionId)
		if !found {
			log.Warn().Str("player", p.Name()).Msgf("starting gear %s/%s not defined, skipping it",
				item.ZoneId, item.DefinitionId)
			continue
		}

		inst := object.NewInstance(uuid.New(), d)
		inst.Power = item.Power
		p.inventory.Add(inst)
		if item.Backfill {
			// handed out now, so the login that follows doesn't again
			p.MarkBackfilled(item.Ref())
		}

		if !item.Equip {
			continue
		}
		slot := d.EquipmentSlot
		if slot == rules.SlotNone {
			log.Warn().Str("player", p.Name()).Msgf("starting gear %s/%s is marked equip but isn't wearable",
				item.ZoneId, item.DefinitionId)
			continue
		}
		if p.equipment.Equipped(slot) {
			log.Warn().Str("player", p.Name()).Msgf("starting gear %s/%s: slot %s is already worn, leaving it in inventory",
				item.ZoneId, item.DefinitionId, slot)
			continue
		}
		p.equipment.Equip(slot, inst)
	}
}
