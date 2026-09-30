package player

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// fakeDefs is a DefinitionSource over a handful of definitions, keyed the way
// the world keys them.
type fakeDefs map[string]*object.Definition

func (f fakeDefs) ObjectDefinition(zoneId, definitionId string) (*object.Definition, bool) {
	d, found := f[zoneId+":"+definitionId]
	return d, found
}

func (f fakeDefs) add(t *testing.T, id string, slot rules.EquipmentSlot) {
	t.Helper()
	f["wrathrock:"+id] = object.NewDefinition(id, id,
		"wrathrock",
		rules.ObjectCategoryOther,
		nil,
		id,
		id+" is here.",
		slot,
		rules.ArmorTypeNone,
		[]rules.ObjectBehavior{})
}

func newTestDefs(t *testing.T) fakeDefs {
	t.Helper()
	defs := fakeDefs{}
	defs.add(t, "knife", rules.SlotWield)
	defs.add(t, "dagger", rules.SlotWield)
	defs.add(t, "tunic", rules.SlotBody)
	defs.add(t, "waterskin", rules.SlotNone)
	return defs
}

func TestGiveStartingGear(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "newbie", nil)

	GiveStartingGear(p, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "knife", Equip: true},
		{ZoneId: "wrathrock", DefinitionId: "tunic", Equip: true},
		{ZoneId: "wrathrock", DefinitionId: "waterskin"},
	}, newTestDefs(t))

	// worn gear is still carried: wearing something never took it out.
	assert.Equal(t, 3, p.Inventory().Len())

	knife := p.Equipment().At(rules.SlotWield)
	require.NotNil(t, knife)
	assert.Equal(t, "knife", knife.Definition.Name)

	tunic := p.Equipment().At(rules.SlotBody)
	require.NotNil(t, tunic)
	assert.Equal(t, "tunic", tunic.Definition.Name)

	carried, found := p.Inventory().Get(knife.Id)
	assert.True(t, found)
	assert.Same(t, knife, carried)

	assert.Nil(t, p.Equipment().At(rules.SlotHead))
}

func TestGiveStartingGear_noGear(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "newbie", nil)
	GiveStartingGear(p, nil, newTestDefs(t))
	assert.Equal(t, 0, p.Inventory().Len())
}

// content that moved underneath the kit shouldn't leave a character with
// nothing at all -- the rest of the kit still arrives.
func TestGiveStartingGear_unknownDefinitionIsSkipped(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "newbie", nil)

	GiveStartingGear(p, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "excalibur", Equip: true},
		{ZoneId: "wrathrock", DefinitionId: "knife", Equip: true},
	}, newTestDefs(t))

	assert.Equal(t, 1, p.Inventory().Len())
	require.NotNil(t, p.Equipment().At(rules.SlotWield))
}

// the loader refuses this at startup; if one ever gets here anyway, the
// second item is carried rather than replacing the first.
func TestGiveStartingGear_slotAlreadyWorn(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "newbie", nil)

	GiveStartingGear(p, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "knife", Equip: true},
		{ZoneId: "wrathrock", DefinitionId: "dagger", Equip: true},
	}, newTestDefs(t))

	assert.Equal(t, 2, p.Inventory().Len())
	wielded := p.Equipment().At(rules.SlotWield)
	require.NotNil(t, wielded)
	assert.Equal(t, "knife", wielded.Definition.Name)
}

func TestGiveStartingGear_equipUnwearableIsCarried(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "newbie", nil)

	GiveStartingGear(p, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "waterskin", Equip: true},
	}, newTestDefs(t))

	assert.Equal(t, 1, p.Inventory().Len())
	assert.Nil(t, p.Equipment().At(rules.SlotNone))
}

// The kit's power is stamped on each item as it is made, so a new character
// starts at whatever the kit says rather than at zero.
func TestGiveStartingGear_power(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "newbie", nil)

	GiveStartingGear(p, rules.StartingGear{
		{ZoneId: "wrathrock", DefinitionId: "knife", Equip: true, Power: 1},
		{ZoneId: "wrathrock", DefinitionId: "tunic", Equip: true, Power: 1},
		{ZoneId: "wrathrock", DefinitionId: "waterskin"},
	}, newTestDefs(t))

	assert.Equal(t, 1, p.Equipment().At(rules.SlotWield).Power)
	assert.Equal(t, 1, p.Power())
}
