package mongostore

import (
	"fmt"
	"time"
	"uuid"

	"github.com/watchmud/watchmud/player"
)

// playerDoc is one character as it sits in the collection.
//
// It exists instead of bson tags on player.Record for two reasons. The
// obvious one is that player/ has no business knowing what database anyone
// chose; the store is an interface precisely so that stays true. The other is
// that uuid.UUID is a [16]byte, and bson would faithfully write it as an
// array of sixteen numbers -- correct, and unreadable the first time you open
// a shell and look at what you saved. Ids are strings here.
type playerDoc struct {
	// Id is the character's uuid, which is the identity that survives
	// everything, including a rename we don't have a command for yet. The
	// name is what you log in with, so it gets a unique index instead.
	Id           string `bson:"_id"`
	Name         string `bson:"name"`
	PasswordHash string `bson:"password_hash"`
	Wizard       bool   `bson:"wizard,omitempty"`
	Bot          bool   `bson:"bot,omitempty"`
	NoColor      bool   `bson:"no_color,omitempty"`
	Coins        int    `bson:"coins,omitempty"`
	CurHealth    int    `bson:"cur_health"`
	MaxHealth    int    `bson:"max_health"`
	CurMana      *int   `bson:"cur_mana,omitempty"`

	LineageId  string   `bson:"lineage_id"`
	LastZoneId string   `bson:"last_zone_id"`
	LastRoomId string   `bson:"last_room_id"`
	Backfilled []string `bson:"backfilled,omitempty"`

	Equipment []equipmentDoc `bson:"equipment"`
	Inventory []inventoryDoc `bson:"inventory"`

	// UpdatedAt is for whoever is reading the collection, not for the game:
	// nothing loads it. It is the cheapest way to answer "is this thing
	// actually writing?" from a mongo shell.
	UpdatedAt time.Time `bson:"updated_at"`
}

type equipmentDoc struct {
	Slot       string `bson:"slot"`
	InstanceId string `bson:"instance_id"`
}

type inventoryDoc struct {
	InstanceId   string `bson:"instance_id"`
	ZoneId       string `bson:"zone_id"`
	DefinitionId string `bson:"definition_id"`

	// Durability is omitted for gear that doesn't wear out, and missing from
	// every document written before durability existed. Both mean the same
	// thing on the way back in: as new. It is not zero, which is broken.
	Durability *int `bson:"durability,omitempty"`

	// Power is omitted at zero, and missing from every document written
	// before power existed. Both come back as power 0.
	Power *int `bson:"power,omitempty"`

	// Contents is a bag's, omitted for everything else.
	Contents []inventoryDoc `bson:"contents,omitempty"`
}

func inventoryDocs(items []player.InventoryRecord) []inventoryDoc {
	var docs []inventoryDoc
	for _, i := range items {
		docs = append(docs, inventoryDoc{
			InstanceId:   i.InstanceId.String(),
			ZoneId:       i.ZoneId,
			DefinitionId: i.DefinitionId,
			Durability:   i.Durability,
			Power:        i.Power,
			Contents:     inventoryDocs(i.Contents),
		})
	}
	return docs
}

func inventoryRecords(name string, docs []inventoryDoc) ([]player.InventoryRecord, error) {
	var records []player.InventoryRecord
	for _, i := range docs {
		instanceId, err := uuid.Parse(i.InstanceId)
		if err != nil {
			return nil, fmt.Errorf("player %s: item %s/%s: bad instance id %q: %w",
				name, i.ZoneId, i.DefinitionId, i.InstanceId, err)
		}
		contents, err := inventoryRecords(name, i.Contents)
		if err != nil {
			return nil, err
		}
		records = append(records, player.InventoryRecord{
			InstanceId:   instanceId,
			ZoneId:       i.ZoneId,
			DefinitionId: i.DefinitionId,
			Durability:   i.Durability,
			Power:        i.Power,
			Contents:     contents,
		})
	}
	return records, nil
}

// newPlayerDoc converts a record on its way to the database.
func newPlayerDoc(r *player.Record, now time.Time) playerDoc {
	doc := playerDoc{
		Id:           r.Id.String(),
		Name:         r.Name,
		PasswordHash: r.PasswordHash,
		Wizard:       r.Wizard,
		Bot:          r.Bot,
		NoColor:      r.NoColor,
		Coins:        r.Coins,
		CurHealth:    r.CurHealth,
		MaxHealth:    r.MaxHealth,
		CurMana:      r.CurMana,
		LineageId:    r.LineageId,
		LastZoneId:   r.LastZoneId,
		LastRoomId:   r.LastRoomId,
		Backfilled:   r.Backfilled,
		UpdatedAt:    now.UTC(),
	}
	for _, e := range r.Equipment {
		doc.Equipment = append(doc.Equipment, equipmentDoc{
			Slot:       e.Slot,
			InstanceId: e.InstanceId.String(),
		})
	}
	doc.Inventory = inventoryDocs(r.Inventory)
	return doc
}

// record converts a document on its way back out.
//
// An id that doesn't parse is an error rather than a fresh uuid: a login that
// quietly renumbered a character's items would reattach their equipment to
// nothing, and the empty slots would look exactly like the content edit that
// player.FromRecord already forgives.
func (d playerDoc) record() (*player.Record, error) {
	id, err := uuid.Parse(d.Id)
	if err != nil {
		return nil, fmt.Errorf("player %s: bad _id %q: %w", d.Name, d.Id, err)
	}
	r := &player.Record{
		Id:           id,
		Name:         d.Name,
		PasswordHash: d.PasswordHash,
		Wizard:       d.Wizard,
		Bot:          d.Bot,
		NoColor:      d.NoColor,
		Coins:        d.Coins,
		CurHealth:    d.CurHealth,
		MaxHealth:    d.MaxHealth,
		CurMana:      d.CurMana,
		LineageId:    d.LineageId,
		LastZoneId:   d.LastZoneId,
		LastRoomId:   d.LastRoomId,
		Backfilled:   d.Backfilled,
	}
	for _, e := range d.Equipment {
		instanceId, err := uuid.Parse(e.InstanceId)
		if err != nil {
			return nil, fmt.Errorf("player %s: slot %s: bad instance id %q: %w", d.Name, e.Slot, e.InstanceId, err)
		}
		r.Equipment = append(r.Equipment, player.EquipmentRecord{
			Slot:       e.Slot,
			InstanceId: instanceId,
		})
	}
	inventory, err := inventoryRecords(d.Name, d.Inventory)
	if err != nil {
		return nil, err
	}
	r.Inventory = inventory
	return r, nil
}
