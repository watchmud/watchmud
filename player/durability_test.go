package player

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// durabilityDefs is a DefinitionSource whose gear wears out.
type durabilityDefs struct {
	knife *object.Definition
}

func newDurabilityDefs(t *testing.T, maxDurability int) durabilityDefs {
	t.Helper()
	knife := object.MakeTestKnife(t)
	def := knife.Definition
	def.MaxDurability = maxDurability
	return durabilityDefs{knife: def}
}

func (d durabilityDefs) ObjectDefinition(zoneId, definitionId string) (*object.Definition, bool) {
	if zoneId == "wrathrock" && definitionId == "knife" {
		return d.knife, true
	}
	return nil, false
}

func worn(t *testing.T, defs durabilityDefs, durability int) *Record {
	t.Helper()
	p := NewTestPlayer(uuid.New(), "scuffed", nil)
	knife := object.NewInstance(uuid.New(), defs.knife)
	knife.Durability = durability
	p.Inventory().Add(knife)
	p.Equipment().Equip(rules.SlotWield, knife)
	return p.Record()
}

// What a character has been through comes back with them.
func TestRecord_durabilityRoundTrips(t *testing.T) {
	defs := newDurabilityDefs(t, 40)
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	rec := worn(t, defs, 12)
	require.NotNil(t, rec.Inventory[0].Durability)
	assert.Equal(t, 12, *rec.Inventory[0].Durability)

	back, err := FromRecord(rec, &Recorder{}, cat, defs)
	require.NoError(t, err)

	knife := back.Equipment().At(rules.SlotWield)
	require.NotNil(t, knife)
	assert.Equal(t, 12, knife.Durability)
	assert.False(t, knife.Broken())
}

// Broken stays broken across a logout. This is the one that a nil-means-new
// rule has to get right in the other direction.
func TestRecord_brokenStaysBroken(t *testing.T) {
	defs := newDurabilityDefs(t, 40)
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	back, err := FromRecord(worn(t, defs, 0), &Recorder{}, cat, defs)
	require.NoError(t, err)

	assert.True(t, back.Equipment().At(rules.SlotWield).Broken())
}

// A record written before durability existed says nothing about it, and that
// has to mean "as new". Reading it as zero would break every item everybody
// owns the first time the server restarts.
func TestRecord_missingDurabilityIsAsNew(t *testing.T) {
	defs := newDurabilityDefs(t, 40)
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	rec := worn(t, defs, 12)
	rec.Inventory[0].Durability = nil // an older save

	back, err := FromRecord(rec, &Recorder{}, cat, defs)
	require.NoError(t, err)

	knife := back.Equipment().At(rules.SlotWield)
	assert.Equal(t, 40, knife.Durability)
	assert.False(t, knife.Broken())
}

// Gear that doesn't wear out doesn't write a number at all, so turning
// durability on later starts everyone from full.
func TestRecord_indestructibleGearRecordsNoDurability(t *testing.T) {
	rec := worn(t, newDurabilityDefs(t, rules.Indestructible), 0)
	assert.Nil(t, rec.Inventory[0].Durability)
}

// Content can be retuned downwards; an item in the world shouldn't stay
// tougher than anything you could get today.
func TestRecord_durabilityIsClampedToTheCurrentMax(t *testing.T) {
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)

	rec := worn(t, newDurabilityDefs(t, 80), 80)
	back, err := FromRecord(rec, &Recorder{}, cat, newDurabilityDefs(t, 30))
	require.NoError(t, err)

	assert.Equal(t, 30, back.Equipment().At(rules.SlotWield).Durability)
}
