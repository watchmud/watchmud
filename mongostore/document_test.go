package mongostore

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/player"
)

func intp(i int) *int { return &i }

func testRecord() *player.Record {
	knifeId := uuid.New()
	return &player.Record{
		Id:           uuid.New(),
		Name:         "newbie",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuuvwxyzABCDEFGHIJKLMNOPQRSTUVWXY",
		Wizard:       true,
		Bot:          true,
		NoColor:      true,
		Coins:        42,
		CurHealth:    93,
		MaxHealth:    100,
		CurMana:      intp(55),
		LineageId:    "hill_dwarf",
		LastZoneId:   "wrathrock",
		LastRoomId:   "temple_square",
		Equipment: []player.EquipmentRecord{
			{Slot: "wield", InstanceId: knifeId},
		},
		Inventory: []player.InventoryRecord{
			{InstanceId: knifeId, ZoneId: "wrathrock", DefinitionId: "training_dagger", Durability: intp(17), Power: intp(5)},
			{InstanceId: uuid.New(), ZoneId: "wrathrock", DefinitionId: "waterskin"},
			{InstanceId: uuid.New(), ZoneId: "wrathrock", DefinitionId: "leather_satchel", Contents: []player.InventoryRecord{
				{InstanceId: uuid.New(), ZoneId: "hollowfields", DefinitionId: "goose_feather", Power: intp(1)},
			}},
		},
		Backfilled: []string{"wrathrock/temple_token"},
	}
}

// What goes in comes back out: the equipped instance id has to still match the
// inventory one, or FromRecord leaves the slot empty and the character logs in
// undressed.
func TestPlayerDoc_roundTrip(t *testing.T) {
	rec := testRecord()

	doc := newPlayerDoc(rec, time.Now())
	got, err := doc.record()
	require.NoError(t, err)

	assert.Equal(t, rec, got)
	assert.Equal(t, got.Inventory[0].InstanceId, got.Equipment[0].InstanceId)
}

// A character with nothing is a real character, not an empty document.
func TestPlayerDoc_roundTripEmptyGear(t *testing.T) {
	rec := &player.Record{
		Id:        uuid.New(),
		Name:      "naked",
		CurHealth: 100,
		MaxHealth: 100,
		LineageId: "human",
	}

	got, err := newPlayerDoc(rec, time.Now()).record()
	require.NoError(t, err)
	assert.Equal(t, rec, got)
}

// ids are written as strings so the collection is readable
func TestNewPlayerDoc_idsAreStrings(t *testing.T) {
	rec := testRecord()
	doc := newPlayerDoc(rec, time.Now())

	assert.Equal(t, rec.Id.String(), doc.Id)
	assert.Equal(t, rec.Equipment[0].InstanceId.String(), doc.Equipment[0].InstanceId)
	assert.Equal(t, rec.Inventory[0].InstanceId.String(), doc.Inventory[0].InstanceId)
}

func TestNewPlayerDoc_updatedAtIsUTC(t *testing.T) {
	doc := newPlayerDoc(testRecord(), time.Date(2026, 9, 21, 12, 0, 0, 0, time.Local))
	assert.Equal(t, time.UTC, doc.UpdatedAt.Location())
}

// a document somebody edited by hand shouldn't come back as a character with
// renumbered gear.
func TestPlayerDoc_badIdIsAnError(t *testing.T) {
	doc := newPlayerDoc(testRecord(), time.Now())
	doc.Id = "not-a-uuid"

	_, err := doc.record()
	assert.ErrorContains(t, err, "bad _id")
}

func TestPlayerDoc_badInstanceIdIsAnError(t *testing.T) {
	doc := newPlayerDoc(testRecord(), time.Now())
	doc.Inventory[1].InstanceId = "nope"

	_, err := doc.record()
	assert.ErrorContains(t, err, "bad instance id")
}

// and inside a bag
func TestPlayerDoc_badInstanceIdInABagIsAnError(t *testing.T) {
	doc := newPlayerDoc(testRecord(), time.Now())
	doc.Inventory[2].Contents[0].InstanceId = "nope"

	_, err := doc.record()
	assert.ErrorContains(t, err, "bad instance id")
}

// Durability rides along with the item it belongs to, and gear that doesn't
// wear out writes no number rather than a zero -- zero is broken.
func TestPlayerDoc_durability(t *testing.T) {
	rec := testRecord()
	doc := newPlayerDoc(rec, time.Now())

	require.NotNil(t, doc.Inventory[0].Durability)
	assert.Equal(t, 17, *doc.Inventory[0].Durability)
	assert.Nil(t, doc.Inventory[1].Durability)

	got, err := doc.record()
	require.NoError(t, err)
	assert.Equal(t, rec, got)
}

// A document written before durability existed has no durability key, which
// has to come back as nil and not as a broken item.
func TestPlayerDoc_documentWithoutDurability(t *testing.T) {
	doc := newPlayerDoc(testRecord(), time.Now())
	doc.Inventory[0].Durability = nil

	got, err := doc.record()
	require.NoError(t, err)
	assert.Nil(t, got.Inventory[0].Durability)
}

// Power rides along the same way, and a document without it -- written before
// power existed, or for power 0 -- comes back as nil, which FromRecord reads
// as 0.
func TestPlayerDoc_power(t *testing.T) {
	rec := testRecord()
	doc := newPlayerDoc(rec, time.Now())

	require.NotNil(t, doc.Inventory[0].Power)
	assert.Equal(t, 5, *doc.Inventory[0].Power)
	assert.Nil(t, doc.Inventory[1].Power)

	got, err := doc.record()
	require.NoError(t, err)
	assert.Equal(t, rec, got)
}
