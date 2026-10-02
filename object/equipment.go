package object

import (
	"iter"
	"maps"
	"slices"

	"github.com/watchmud/watchmud/rules"
)

type Equipment struct {
	eqMap map[rules.EquipmentSlot]*Instance

	// What gear is worth -- the armor table and which roles armor argues for
	// -- lives in the catalog, so equipment has to be able to ask. This is
	// not the same as holding a role: nothing here is stored, every answer is
	// recomputed from what is in the slots right now.
	cat *rules.Catalog
}

func NewEquipment(cat *rules.Catalog) *Equipment {
	return &Equipment{
		eqMap: make(map[rules.EquipmentSlot]*Instance),
		cat:   cat,
	}
}

// All the equipped items, slot and instance, in the order a player expects to
// read them -- see rules.CompareSlots. Empty slots are skipped, so nobody has
// to remember the nil check.
func (eq *Equipment) All() iter.Seq2[rules.EquipmentSlot, *Instance] {
	return func(yield func(rules.EquipmentSlot, *Instance) bool) {
		for _, slot := range eq.slots() {
			if inst := eq.eqMap[slot]; inst != nil {
				if !yield(slot, inst) {
					return
				}
			}
		}
	}
}

// slots that currently hold something, in canonical order.
func (eq *Equipment) slots() []rules.EquipmentSlot {
	slots := slices.Collect(maps.Keys(eq.eqMap))
	slices.SortFunc(slots, rules.CompareSlots)
	return slots
}

func (eq *Equipment) At(slot rules.EquipmentSlot) *Instance {
	return eq.eqMap[slot]
}

func (eq *Equipment) Equip(slot rules.EquipmentSlot, obj *Instance) {
	eq.eqMap[slot] = obj
}

func (eq *Equipment) Unequip(slot rules.EquipmentSlot) {
	eq.eqMap[slot] = nil
}

func (eq *Equipment) Find(target string) (rules.EquipmentSlot, *Instance, bool) {
	for slot, inst := range eq.All() {
		if inst.Matches(target) {
			return slot, inst, true
		}
	}
	return rules.SlotNone, nil, false
}

func (eq *Equipment) Equipped(slot rules.EquipmentSlot) bool {
	return eq.eqMap[slot] != nil
}

func (eq *Equipment) ItemEquipped(item *Instance) bool {
	if item == nil {
		return false
	}
	for _, inst := range eq.eqMap {
		if inst == item {
			return true
		}
	}
	return false
}

// ArmorClass sums what's equipped, asking the armor table what each piece is
// worth in the slot it's in, starting from rules.BaseArmorClass. Anything
// that isn't armor adds nothing, and neither does anything broken.
func (eq *Equipment) ArmorClass() int {
	ac := rules.BaseArmorClass
	for slot, inst := range eq.All() {
		if inst.Broken() {
			continue
		}
		ac += eq.cat.ArmorWeight(inst.Definition.ArmorType, slot)
	}
	return ac
}

// Power is the average power of what's equipped, rounded down, and it is the
// whole of a player's level -- see LEVELS.md. Empty slots don't count, and
// broken gear doesn't either: not as a zero, which would make wearing it worse
// than wearing nothing, but not at all. Nothing on is power 0.
func (eq *Equipment) Power() int {
	total, count := 0, 0
	for _, inst := range eq.All() {
		if inst.Broken() {
			continue
		}
		total += inst.Power
		count++
	}
	if count == 0 {
		return 0
	}
	return total / count
}

// DamageableGear is everything worn that could still take wear: it has
// durability, and it hasn't already given out. In slot order.
//
// This is what dying costs you -- all of it, weapon included, since the whole
// kit was there when it happened.
func (eq *Equipment) DamageableGear() []*Instance {
	var pieces []*Instance
	for _, inst := range eq.All() {
		if !inst.WearsOut() || inst.Broken() {
			continue
		}
		pieces = append(pieces, inst)
	}
	return pieces
}

// DamageableArmor is the same list narrowed to what stops a blow: armor, by
// virtue of having an armor type. A censer is not what a sword lands on.
// Picking one out of this is reproducible given the same roll, because the
// order is the slot order.
func (eq *Equipment) DamageableArmor() []*Instance {
	var pieces []*Instance
	for _, inst := range eq.DamageableGear() {
		if inst.Definition.ArmorType == rules.ArmorTypeNone {
			continue
		}
		pieces = append(pieces, inst)
	}
	return pieces
}

// RoleWeights totals what everything equipped contributes to each role,
// keyed on rules.Role.Id. This is the whole input to a character's role:
// change what's in the slots and the totals change with it.
//
// Returned without resolving to a rules.Role, because player has no business
// holding the catalog -- the handler that needs a name calls
// rules.Catalog.RoleFor with this.
func (eq *Equipment) RoleWeights() map[string]int {
	totals := make(map[string]int)
	for _, c := range eq.RoleContributions() {
		totals[c.RoleId] += c.Weight
	}
	return totals
}

// A RoleContribution is one equipped item's argument for one role: what it is,
// which role it speaks for and how loudly.
type RoleContribution struct {
	Slot     rules.EquipmentSlot
	Instance *Instance
	RoleId   string
	Weight   int
}

// RoleContributions is every such argument, in slot order, and within one item
// in the catalog's role order. The totals and the "why" a player is shown come
// from this one list, so the reasons always add up to the number beside them.
//
// Two sources: what an object declares by hand, and what its armor type is
// worth in the slot it's worn in. A piece of armor that also declares weights
// gets both, which is how a magic helmet argues for something armor alone
// wouldn't.
func (eq *Equipment) RoleContributions() []RoleContribution {
	var contributions []RoleContribution
	for slot, inst := range eq.All() {
		// Broken gear argues for nothing. It is still on you, and a player
		// whose breastplate gave out is a player who has stopped being a
		// Tank -- which is the point: the gear is the truth, and broken gear
		// tells the truth about what it is doing for you.
		if inst.Broken() {
			continue
		}
		for _, roleId := range slices.Sorted(maps.Keys(inst.Definition.RoleWeights)) {
			if weight := inst.Definition.RoleWeights[roleId]; weight != 0 {
				contributions = append(contributions, RoleContribution{
					Slot: slot, Instance: inst, RoleId: roleId, Weight: weight,
				})
			}
		}
		armor := eq.cat.ArmorWeight(inst.Definition.ArmorType, slot)
		if armor == 0 {
			continue
		}
		for _, r := range eq.cat.ArmorRoles() {
			contributions = append(contributions, RoleContribution{
				Slot: slot, Instance: inst, RoleId: r.Id, Weight: armor,
			})
		}
	}
	return contributions
}

// Grant is the item an ability comes from, and the power it is cast at.
type Grant struct {
	Instance *Instance
	Power    int
}

// Abilities is what the equipped, unbroken gear lets the wearer cast, keyed
// on ability id. Where two items grant the same ability, the stronger is
// what you cast with; on a tie, the first in slot order. Recomputed every
// call, like role and power: nothing records what a player can do.
//
// A map, since only the listing cares about order, and that comes from the
// catalog - see world.handleAbilities.
func (eq *Equipment) Abilities() map[string]Grant {
	grants := make(map[string]Grant)
	for _, inst := range eq.All() {
		if inst.Broken() {
			continue
		}
		for _, id := range inst.Definition.Abilities {
			if g, have := grants[id]; !have || inst.Power > g.Power {
				grants[id] = Grant{Instance: inst, Power: inst.Power}
			}
		}
	}
	return grants
}
