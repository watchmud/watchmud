package player

import (
	"errors"
	"slices"
	"uuid"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type Record struct {
	Id                   uuid.UUID
	Name                 string
	PasswordHash         string
	Wizard               bool
	Bot                  bool
	NoColor              bool // inverted: a record from before the choice is color on
	NoOOC                bool // inverted the same way: everyone starts on the channel
	Coins                int
	CurHealth, MaxHealth int
	// CurMana is a pointer for the durability reason: a record from before
	// mana existed reads as full, not empty. Max isn't saved; it's
	// rules.MaxMana for everyone.
	CurMana                *int
	LineageId              string // cosmetic; there is no ClassId beside it any more
	LastZoneId, LastRoomId string
	Equipment              []EquipmentRecord
	Inventory              []InventoryRecord
	// Backfilled is every starting-gear item marked backfill this character
	// has been handed, as "zone/object": once each, at creation or at the
	// first login after the item joined the kit.
	Backfilled []string
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

	// Contents is what's in it, for a bag. One level: nothing that holds
	// things goes inside another.
	Contents []InventoryRecord
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
	if rec.CurMana != nil {
		// absent is a record from before mana, which leaves New's full pool
		p.curMana = min(max(*rec.CurMana, 0), p.maxMana)
	}
	p.wizard = rec.Wizard
	p.bot = rec.Bot
	p.noColor = rec.NoColor
	p.noOOC = rec.NoOOC
	p.coins = max(rec.Coins, 0)
	p.backfilled = slices.Clone(rec.Backfilled)

	for _, ir := range rec.Inventory {
		i, ok := instanceFromRecord(rec.Name, ir, defs)
		if !ok {
			continue
		}
		if err := p.inventory.Add(i); err != nil {
			p.Log().Error().Err(err).Msg("loading inventory")
		}
		for _, cr := range ir.Contents {
			c, ok := instanceFromRecord(rec.Name, cr, defs)
			if !ok {
				continue
			}
			// a bag content no longer calls a container: what was in it is
			// carried loose rather than lost
			into := p.inventory
			if i.Contents != nil {
				into = i.Contents
			}
			if err := into.Add(c); err != nil {
				p.Log().Error().Err(err).Msg("loading a bag")
			}
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

// instanceFromRecord makes the item a record describes, or says it can't.
//
// Missing definitions. A saved InventoryRecord can reference a zone or object
// id that content no longer defines -- you edit content/, and last week's save
// now points at nothing. Erroring means an unlucky content edit locks a player
// out of the game permanently. Log and skip the item so the login succeeds;
// that's the behavior you want at 2am.
func instanceFromRecord(name string, ir InventoryRecord, defs DefinitionSource) (*object.Instance, bool) {
	d, found := defs.ObjectDefinition(ir.ZoneId, ir.DefinitionId)
	if !found {
		log.Warn().Str("player", name).Msgf("definition not found for %s / %s, dropping it", ir.ZoneId, ir.DefinitionId)
		return nil, false
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
	return i, true
}

func (p *Player) Record() *Record {
	mana := p.curMana
	return &Record{
		Id:           p.Id(),
		Name:         p.Name(),
		PasswordHash: p.passwordHash,
		Wizard:       p.wizard,
		Bot:          p.bot,
		NoColor:      p.noColor,
		NoOOC:        p.noOOC,
		Coins:        p.coins,
		CurHealth:    p.curHealth,
		MaxHealth:    p.maxHealth,
		LineageId:    p.Lineage.Id,
		Equipment:    EquipmentToRecord(p.equipment),
		Inventory:    inventoryRecord(p.inventory),
		CurMana:      &mana,
		Backfilled:   slices.Clone(p.backfilled),
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
		if item.Contents != nil {
			r.Contents = inventoryRecord(item.Contents)
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
