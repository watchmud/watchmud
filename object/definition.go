package object

import (
	"slices"
	"strings"

	"github.com/watchmud/watchmud/rules"
)

// Definition of what it means to be an "object"
// the "platonic form" of an object, if you will.
//
// There is a definition of the type "ShinySword" which defines the
// properties of what it means to be a ShinySword. An actual
// ShinySword lying around is an Instance.
type Definition struct {
	ObjectId            Id
	Aliases             []string
	ObjectCategory      rules.ObjectCategory
	Name                string
	ShortDescription    string // description of the object when being used: "a long, green stick" -> "The Beastly Fido picks up the long, green stick."
	DescriptionOnGround string // description of the object when lying on the ground: "A shiny sword is lying here."
	EquipmentSlot       rules.EquipmentSlot
	Behaviors           []rules.ObjectBehavior
	ArmorType           rules.ArmorType

	// MaxDurability is what one of these starts out able to take, and what a
	// repair would restore it to. rules.Indestructible (zero) means it never
	// wears out, which is what everything is until content says otherwise.
	//
	// Assigned by the loader, like RoleWeights and for the same two reasons:
	// NewDefinition has enough positional arguments, and the number is
	// usually derived from the durability table rather than written on the
	// object.
	MaxDurability int

	// Damage is what this hits with when wielded. Empty for anything that
	// isn't wielded; the loader refuses a wielded object without it. Assigned
	// by the loader, like MaxDurability.
	Damage rules.DamageRoll

	// RoleWeights is what this object contributes to each role while
	// equipped, keyed on rules.Role.Id: {"striker": 2}. Hand-authored in
	// objects.json, and on its way out for armor, which can say "plate" and
	// let rules.Catalog.Armor do the arithmetic -- but a knife or a censer
	// has no armor type to derive anything from, so this is still how a
	// weapon argues for a role. Assigned by the loader rather than passed to
	// NewDefinition, which has enough positional arguments already.
	RoleWeights map[string]int
}

func NewDefinition(
	id string,
	name string,
	zoneId string,
	category rules.ObjectCategory,
	aliases []string,
	shortDescription string,
	descriptionOnGround string,
	equipmentSlot rules.EquipmentSlot,
	armorType rules.ArmorType,
	behaviors []rules.ObjectBehavior) *Definition {
	d := &Definition{
		ObjectId:            NewObjectId(id, zoneId),
		Name:                strings.ToLower(name),
		ShortDescription:    shortDescription,
		DescriptionOnGround: descriptionOnGround,
		ObjectCategory:      category,
		Aliases:             aliases,
		EquipmentSlot:       equipmentSlot,
		ArmorType:           armorType,
		Behaviors:           behaviors,
	}
	return d
}

func (d *Definition) IsWeapon() bool {
	return d.ObjectCategory == rules.ObjectCategoryWeapon
}

func (d *Definition) NoTake() bool {
	return slices.Contains(d.Behaviors, rules.ObjectBehaviorNoTake)
}

func (d *Definition) Takeable() bool {
	return !d.NoTake()
}

func (d *Definition) Wearable() bool {
	return d.EquipmentSlot != rules.SlotNone
}

func (d *Definition) HasAlias(target string) bool {
	return slices.Contains(d.Aliases, target)
}

func (d *Definition) Matches(target string) bool {
	target = strings.ToLower(target)
	if d.Name == target || d.HasAlias(target) {
		return true
	}
	return false
}
