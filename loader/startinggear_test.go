package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
	"github.com/watchmud/watchmud/zonereset"
)

func TestLoadContent_startingGear(t *testing.T) {
	c, err := LoadContent(os.DirFS("../testcontent"))
	require.NoError(t, err)

	assert.Equal(t, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "knife", Equip: true},
		{ZoneId: "wrathrock", DefinitionId: "iron_helmet", Equip: true},
		{ZoneId: "wrathrock", DefinitionId: "rope"},
	}, c.Catalog.StartingGear)
}

// contentWithGear is a world of one zone holding a knife, a helmet and a
// fountain nobody can wear, which is enough to be wrong in every way the
// check cares about.
func contentWithGear(t *testing.T, gear rules.StartingGear) *Content {
	t.Helper()

	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)
	cat.StartingGear = gear
	zone := spaces.NewZone("wrathrock", "Wrathrock", zonereset.NEVER, 0)
	zone.AddObjectDefinition(object.NewDefinition("knife", "knife", "wrathrock",
		rules.ObjectCategoryWeapon, nil, "knife", "A knife is here.", rules.SlotWield, rules.ArmorTypeNone, rules.EmptyObjectBehaviors))
	zone.AddObjectDefinition(object.NewDefinition("dagger", "dagger", "wrathrock",
		rules.ObjectCategoryWeapon, nil, "dagger", "A dagger is here.", rules.SlotWield, rules.ArmorTypeNone, rules.EmptyObjectBehaviors))
	zone.AddObjectDefinition(object.NewDefinition("fountain", "fountain", "wrathrock",
		rules.ObjectCategoryOther, nil, "fountain", "A fountain bubbles.", rules.SlotNone, rules.ArmorTypeNone, rules.EmptyObjectBehaviors))

	return NewContent(&Settings{}, cat, []*spaces.Zone{zone})
}

func TestCheckStartingGear_ok(t *testing.T) {
	c := contentWithGear(t, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "knife", Equip: true},
		{ZoneId: "wrathrock", DefinitionId: "fountain"},
	})
	assert.NoError(t, c.checkStartingGear())
}

func TestCheckStartingGear_noGearIsFine(t *testing.T) {
	c := contentWithGear(t, nil)
	assert.NoError(t, c.checkStartingGear())
}

func TestCheckStartingGear_unknownZone(t *testing.T) {
	c := contentWithGear(t, rules.StartingGear{
		{ZoneId: "atlantis", DefinitionId: "knife", Equip: true},
	})
	assert.ErrorContains(t, c.checkStartingGear(), "zone not found")
}

func TestCheckStartingGear_unknownObject(t *testing.T) {
	c := contentWithGear(t, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "excalibur", Equip: true},
	})
	assert.ErrorContains(t, c.checkStartingGear(), "not defined in that zone")
}

func TestCheckStartingGear_equipUnwearable(t *testing.T) {
	c := contentWithGear(t, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "fountain", Equip: true},
	})
	assert.ErrorContains(t, c.checkStartingGear(), "no equipment_slot")
}

// two items worn in one slot would leave one of them quietly in inventory,
// which reads as gear that silently didn't work.
func TestCheckStartingGear_slotClaimedTwice(t *testing.T) {
	c := contentWithGear(t, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "knife", Equip: true},
		{ZoneId: "wrathrock", DefinitionId: "dagger", Equip: true},
	})
	assert.ErrorContains(t, c.checkStartingGear(), "slot wield is already worn by knife")
}

// carrying two of the same thing is not a slot conflict.
func TestCheckStartingGear_sameSlotNotEquipped(t *testing.T) {
	c := contentWithGear(t, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "knife", Equip: true},
		{ZoneId: "wrathrock", DefinitionId: "dagger"},
	})
	assert.NoError(t, c.checkStartingGear())
}

func TestCheckStartingGear_negativePower(t *testing.T) {
	c := contentWithGear(t, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "knife", Equip: true, Power: -1},
	})
	assert.ErrorContains(t, c.checkStartingGear(), "negative power")
}

// The real kit starts a new character level with the weakest thing in the
// Hollowfields, not below it.
func TestLoadContent_startingGearMatchesTheNewbieZone(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)

	for _, item := range c.Catalog.StartingGear {
		if item.Equip {
			assert.Equal(t, c.Zones["hollowfield"].Power.Min, item.Power, "%s/%s", item.ZoneId, item.DefinitionId)
		}
	}
}
