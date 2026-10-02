package rules

import "fmt"

// Catalog holds the static game content loaded from the world files. It is
// built once at startup and treated as read-only thereafter.
type Catalog struct {
	Species  map[string]*Species
	Lineages map[string]*Lineage
	Roles    map[string]*Role
	Armor    ArmorTypeContent

	// MudTime holds the constants used to derive PulseCount values.
	MudTime MudTime

	// Durability is what gear starts out able to take. Like StartingGear it
	// is assigned by the loader; unlike it, an empty table is not an empty
	// feature -- it means nothing in the game wears out.
	Durability DurabilityTable

	// Economy is what things cost. Assigned by the loader; the zero value is
	// the economy switched off. See Economy.
	Economy Economy

	// Abilities are what gear can let a player do, keyed on id. Assigned by
	// the loader through SetAbilities; none is a game where nobody casts.
	Abilities map[string]*Ability

	// StartingGear is what a new character is created holding. Assigned by
	// the loader rather than passed to NewCatalog, for the same reason
	// object.Definition.RoleWeights is: it is content that has to be checked
	// against the zones, and the zones aren't loaded yet when the catalog is
	// built. A catalog with none is a game that hands out nothing.
	StartingGear StartingGear

	// declaration order, preserved from the content files: the creation menu
	// reads speciesOrder, and roleOrder breaks ties in RoleFor. Iterating the
	// maps instead would shuffle both from run to run.
	speciesOrder []*Species
	roleOrder    []*Role

	abilityOrder []*Ability

	// the roles armor feeds, in declaration order
	armorRoles []*Role
}

func NewCatalog(
	mudTime MudTime,
	species []*Species,
	roles []*Role,
	armor ArmorTypeContent,
) (*Catalog, error) {
	speciesMap, lineageMap, err := indexSpecies(species)
	if err != nil {
		return nil, err
	}

	roleMap, roleOrder, err := indexRoles(roles)
	if err != nil {
		return nil, err
	}

	var armorRoles []*Role
	for _, r := range roleOrder {
		if r.FromArmor {
			armorRoles = append(armorRoles, r)
		}
	}

	return &Catalog{
		MudTime:      mudTime,
		Species:      speciesMap,
		Lineages:     lineageMap,
		Roles:        roleMap,
		Armor:        armor,
		speciesOrder: species,
		roleOrder:    roleOrder,
		armorRoles:   armorRoles,
	}, nil
}

// IndexSpecies wires up parent pointers and builds the lookup maps, failing
// on duplicate or missing ids. Lineage ids must be unique across all species,
// since that is what gets stored against a player.
func indexSpecies(all []*Species) (map[string]*Species, map[string]*Lineage, error) {
	species := make(map[string]*Species, len(all))
	lineages := make(map[string]*Lineage)

	for _, s := range all {
		if s.Id == "" {
			return nil, nil, fmt.Errorf("species %q: missing id", s.Name)
		}
		if prev, dup := species[s.Id]; dup {
			return nil, nil, fmt.Errorf("duplicate species id %q (%s and %s)", s.Id, prev.Name, s.Name)
		}
		species[s.Id] = s

		if len(s.Lineages) == 0 {
			return nil, nil, fmt.Errorf("species %q: has no lineages", s.Id)
		}
		for _, l := range s.Lineages {
			if l.Id == "" {
				return nil, nil, fmt.Errorf("species %q: lineage %q missing id", s.Id, l.Name)
			}
			if prev, dup := lineages[l.Id]; dup {
				return nil, nil, fmt.Errorf("duplicate lineage id %q (%s and %s)",
					l.Id, prev.Species.Id, s.Id)
			}
			l.Species = s
			lineages[l.Id] = l
		}
	}
	return species, lineages, nil
}

// SpeciesList returns every species in the order the content declared them,
// each still holding its own lineages in their declared order. This is the
// character creation menu.
func (c *Catalog) SpeciesList() []*Species {
	return c.speciesOrder
}

// RoleList returns every role in the order the content declared them.
func (c *Catalog) RoleList() []*Role {
	return c.roleOrder
}

// SetAbilities checks and indexes the abilities, keeping their
// declaration order for listings.
func (c *Catalog) SetAbilities(all []*Ability) error {
	byId := make(map[string]*Ability, len(all))
	for _, a := range all {
		if err := a.check(); err != nil {
			return err
		}
		if _, dup := byId[a.Id]; dup {
			return fmt.Errorf("duplicate ability Id %q", a.Id)
		}
		byId[a.Id] = a
	}
	c.Abilities = byId
	c.abilityOrder = all
	return nil
}

// AbilityList returns every ability in the order the content declared them.
func (c *Catalog) AbilityList() []*Ability {
	return c.abilityOrder
}

// DefaultLineage is what a character gets when nobody picked one, or when the
// one they picked is no longer in the content. Since lineage is cosmetic, an
// unrecognized one is never worth refusing a login over.
func (c *Catalog) DefaultLineage() *Lineage {
	for _, s := range c.speciesOrder {
		if len(s.Lineages) > 0 {
			return s.Lineages[0]
		}
	}
	return nil
}

// ArmorRoles are the roles that armor argues for by itself, declared with
// "from_armor" in roles.json. Usually exactly one; more than one is allowed
// and each gets the full weight, since nothing about the idea says a suit of
// plate can only make you one thing.
func (c *Catalog) ArmorRoles() []*Role {
	if c == nil {
		return nil
	}
	return c.armorRoles
}

// ArmorWeight is what armor of this type is worth in this slot, from
// armor.json: plate on a body is worth more than plate on a hand. Armor with
// no type, or a slot the table doesn't mention, is worth nothing rather than
// an error -- a censer is not bad armor, it is not armor.
//
// The same number is both the AC it adds and what it contributes to the roles
// armor feeds, on purpose: the protection is the argument.
func (c *Catalog) ArmorWeight(t ArmorType, slot EquipmentSlot) int {
	if c == nil || t == ArmorTypeNone {
		return 0
	}
	return c.Armor[t][slot]
}

// RoleFor picks the role a set of equipment adds up to: the highest total
// wins, and a tie goes to whichever role the content declared first. Gear
// that contributes nothing to any role -- or no gear at all -- is no role,
// reported as nil, because "you are wearing nothing in particular" is a real
// answer and inventing a default would hide it.
func (c *Catalog) RoleFor(weights map[string]int) *Role {
	var best *Role
	bestTotal := 0
	for _, r := range c.roleOrder {
		if total := weights[r.Id]; total > bestTotal {
			best, bestTotal = r, total
		}
	}
	return best
}
