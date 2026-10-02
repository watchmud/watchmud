package object

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/rules"
)

// a censer-ish thing for this slot, granting heal at this power
func granting(t *testing.T, name string, slot rules.EquipmentSlot, power int) *Instance {
	t.Helper()
	d := NewTestDefinition(t, name, slot, rules.ObjectCategoryOther, rules.ArmorTypeNone)
	d.Abilities = []string{"heal"}
	d.MaxDurability = 10
	inst := NewInstance(uuid.New(), d)
	inst.Power = power
	return inst
}

func newTestEquipment(t *testing.T) *Equipment {
	t.Helper()
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)
	return NewEquipment(cat)
}

func TestAbilities_nothingEquipped(t *testing.T) {
	assert.Empty(t, newTestEquipment(t).Abilities())
}

func TestAbilities_equippedGrants(t *testing.T) {
	eq := newTestEquipment(t)
	censer := granting(t, "censer", rules.SlotHold, 3)
	eq.Equip(rules.SlotHold, censer)

	assert.Equal(t, map[string]Grant{"heal": {Instance: censer, Power: 3}}, eq.Abilities())
}

// two items granting one ability: the stronger one is what you cast with
func TestAbilities_highestPowerWins(t *testing.T) {
	eq := newTestEquipment(t)
	eq.Equip(rules.SlotNeck, granting(t, "charm", rules.SlotNeck, 2))
	censer := granting(t, "censer", rules.SlotHold, 8)
	eq.Equip(rules.SlotHold, censer)

	assert.Equal(t, Grant{Instance: censer, Power: 8}, eq.Abilities()["heal"])
}

// a tie goes to the first in slot order, not to whichever went on first:
// hold comes before neck (rules.CompareSlots), so the censer wins
func TestAbilities_tieGoesToSlotOrder(t *testing.T) {
	eq := newTestEquipment(t)
	eq.Equip(rules.SlotNeck, granting(t, "charm", rules.SlotNeck, 3))
	censer := granting(t, "censer", rules.SlotHold, 3)
	eq.Equip(rules.SlotHold, censer)

	assert.Equal(t, Grant{Instance: censer, Power: 3}, eq.Abilities()["heal"])
}

// Review Focus 4: a broken censer grants nothing, even if it was the better one
func TestAbilities_brokenGrantsNothing(t *testing.T) {
	eq := newTestEquipment(t)
	charm := granting(t, "charm", rules.SlotNeck, 2)
	eq.Equip(rules.SlotNeck, charm)
	censer := granting(t, "censer", rules.SlotHold, 8)
	censer.Durability = 0
	require.True(t, censer.Broken())
	eq.Equip(rules.SlotHold, censer)

	assert.Equal(t, Grant{Instance: charm, Power: 2}, eq.Abilities()["heal"])

	eq.Unequip(rules.SlotNeck)
	assert.Empty(t, eq.Abilities(), "only the broken one left")
}
