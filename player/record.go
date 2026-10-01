package player

import (
	"errors"
	"uuid"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type Record struct {
	Id                     uuid.UUID
	Name                   string
	PasswordHash           string
	Wizard                 bool
	Bot                    bool
	NoColor                bool // inverted: a record from before the choice is color on
	CurHealth, MaxHealth   int
	LineageId              string // cosmetic; there is no ClassId beside it any more
	LastZoneId, LastRoomId string
	Equipment              []EquipmentRecord
	Inventory              []InventoryRecord
}

type EquipmentRecord struct {
	Slot       string
	InstanceId uuid.UUID
}

type InventoryRecord struct {
	InstanceId   uuid.UUID
	ZoneId       string
	DefinitionId string

	// Durability is what this particular item has left. A pointer because a
	// record written before durability existed has nothing to say about it,
	// and a missing number has to mean "as new" rather than zero -- zero is
	// broken, and a content edit should not break every item everybody owns.
	Durability *int

	// Power is what this particular item is worth. A pointer for the same
	// reason as Durability, though here a missing number and zero agree:
	// before power existed everything was power 0.
	Power *int
}

type DefinitionSource interface {
	ObjectDefinition(zoneId, definitionId string) (*object.Definition, bool)
}

func FromRecord(rec *Record, out Sender, cat *rules.Catalog, defs DefinitionSource) (*Player, error) {
	// Same reasoning as the missing definitions below: a lineage that content
	// no longer defines used to refuse the login outright. It is cosmetic now
	// -- it grants nothing and nothing depends on it -- so a retired lineage
	// is worth a log line and a fallback, never a locked-out player.
	lineage, found := cat.Lineages[rec.LineageId]
	if !found {
		log.Warn().Str("player", rec.Name).Msgf("lineage %q not in catalog, falling back to the default", rec.LineageId)
		lineage = cat.DefaultLineage()
		if lineage == nil {
			return nil, errors.New("no lineages defined in the catalog")
		}
	}
	p := New(rec.Id, rec.Name, rec.PasswordHash, out, lineage, cat)
	p.curHealth = rec.CurHealth
	p.maxHealth = rec.MaxHealth
	p.wizard = rec.Wizard
	p.bot = rec.Bot
	p.noColor = rec.NoColor

	for _, ir := range rec.Inventory {
		//Missing definitions. A saved InventoryRecord can reference a zone or object id that content no longer defines — you edit content/, and last week's save now points at nothing. Erroring means an unlucky
		//content edit locks a player out of the game permanently. Log and skip the item so the login succeeds; that's the behavior you want at 2am.
		d, found := defs.ObjectDefinition(ir.ZoneId, ir.DefinitionId)
		if !found {
			log.Warn().Str("player", rec.Name).Msgf("definition not found for %s / %s, dropping it", ir.ZoneId, ir.DefinitionId)
			continue
		}
		i := object.NewInstance(ir.InstanceId, d)
		if ir.Durability != nil {
			// what it had left when it was saved. Clamped to the current max,
			// so lowering a durability table in content doesn't leave items
			// in the world tougher than anything you can get now.
			i.Durability = min(*ir.Durability, d.MaxDurability)
		}
		if ir.Power != nil {
			// nothing makes negative power; a record claiming it is damaged,
			// and shouldn't drag the average of what the player wears down.
			i.Power = max(*ir.Power, 0)
		}
		if err := p.inventory.Add(i); err != nil {
			p.Log().Error().Err(err).Msg("loading inventory")
		}
	}

	for _, s := range rec.Equipment {
		item, exists := p.inventory.Get(s.InstanceId)
		if !exists {
			// the item didn't survive a content edit; leave the slot empty
			// rather than filling it with a nil that reads as occupied.
			log.Warn().Str("player", rec.Name).Msgf("instance %s not found for slot %s", s.InstanceId, s.Slot)
			continue
		}
		if slot, err := rules.ParseEquipmentSlot(s.Slot); err != nil {
			// the item has a slot that no longer exists, leave it empty and warn.
			log.Warn().Str("player", rec.Name).Msgf("slot %s no longer exists, dropping item %s", s.Slot, item.Id)
			continue
		} else {
			p.equipment.Equip(slot, item)
		}
	}
	return p, nil
}

func (p *Player) Record() *Record {
	return &Record{
		Id:           p.Id(),
		Name:         p.Name(),
		PasswordHash: p.passwordHash,
		Wizard:       p.wizard,
		Bot:          p.bot,
		NoColor:      p.noColor,
		CurHealth:    p.curHealth,
		MaxHealth:    p.maxHealth,
		LineageId:    p.Lineage.Id,
		Equipment:    EquipmentToRecord(p.equipment),
		Inventory:    inventoryRecord(p.inventory),
	}
}

func inventoryRecord(l *object.List) []InventoryRecord {
	var records []InventoryRecord
	for item := range l.All() {
		r := InventoryRecord{
			InstanceId:   item.Id,
			ZoneId:       item.Definition.ObjectId.ZoneId,
			DefinitionId: item.Definition.ObjectId.DefinitionId,
		}
		if item.WearsOut() {
			durability := item.Durability
			r.Durability = &durability
		}
		if item.Power > 0 {
			power := item.Power
			r.Power = &power
		}
		records = append(records, r)
	}
	return records
}

func EquipmentToRecord(eq *object.Equipment) []EquipmentRecord {
	var records []EquipmentRecord
	for slot, item := range eq.All() {
		records = append(records, EquipmentRecord{
			Slot:       string(slot),
			InstanceId: item.Id,
		})
	}
	return records
}
