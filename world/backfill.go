package world

import (
	"uuid"

	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
)

// backfill hands a character any starting gear marked backfill that they
// came too early to be given -- the temple token, to everyone made before
// recall needed one. Once each, by the record's Backfilled list; worn if its
// slot is free, otherwise in the pack. After the room in Arrive, so they read
// where they are before what they've found.
func (w *World) backfill(p *player.Player) {
	for _, item := range w.content.Catalog.StartingGear {
		if !item.Backfill || p.HasBackfilled(item.Ref()) {
			continue
		}
		d, found := w.ObjectDefinition(item.ZoneId, item.DefinitionId)
		if !found {
			// the loader checked the kit; content changing under a running
			// world is the only way here
			p.Log().Warn().Msgf("backfill %s: not defined", item.Ref())
			continue
		}
		inst := object.NewInstance(uuid.New(), d)
		inst.Power = item.Power
		if err := p.Inventory().Add(inst); err != nil {
			p.Log().Error().Err(err).Msgf("backfill %s", item.Ref())
			continue
		}
		worn := item.Equip && d.Wearable() && !p.Equipment().Equipped(d.EquipmentSlot)
		if worn {
			p.Equipment().Equip(d.EquipmentSlot, inst)
		}
		p.MarkBackfilled(item.Ref())
		p.Send(event.Received{Item: d.ShortDescription, Worn: worn})
	}
}
