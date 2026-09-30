package object

import (
	"testing"
	"uuid"

	"github.com/watchmud/watchmud/rules"
)

// MakeTestArmor of this armor type, with this much durability in it
func MakeTestArmor(t *testing.T, slot rules.EquipmentSlot, name string, armorType rules.ArmorType, maxDurability int) *Instance {
	t.Helper()
	d := NewTestDefinition(t, name, slot, rules.ObjectCategoryArmor, armorType)
	d.MaxDurability = maxDurability
	return NewInstance(uuid.New(), d)
}

func MakeTestKnife(t *testing.T) *Instance {
	t.Helper()
	d := NewDefinition("knife",
		"knife",
		"wrathrock",
		rules.ObjectCategoryWeapon,
		nil,
		"An old knife",
		"An old knife is here.",
		rules.SlotWield,
		rules.ArmorTypeNone,
		[]rules.ObjectBehavior{},
	)
	d.Damage = "1d8"
	return NewInstance(uuid.New(), d)
}

func MakeTestFountain(t *testing.T) *Instance {
	t.Helper()
	d := NewDefinition(
		"fountain",
		"fountain",
		"wrathrock",
		rules.ObjectCategoryOther,
		nil,
		"fountain",
		"A fountain bubbles here.",
		rules.SlotNone,
		rules.ArmorTypeNone,
		[]rules.ObjectBehavior{rules.ObjectBehaviorNoTake},
	)
	return NewInstance(uuid.New(), d)
}

func NewTestDefinition(t *testing.T, name string, slot rules.EquipmentSlot, cat rules.ObjectCategory, armorType rules.ArmorType) *Definition {
	t.Helper()
	return NewDefinition(
		name,
		name,
		"zone",
		cat,
		nil, //[]string{"iron", "helm"},
		name,
		name+" is here.",
		slot,
		armorType,
		[]rules.ObjectBehavior{},
	)
}
