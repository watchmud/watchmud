package mobile

import (
	"strings"

	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/rules"
)

// Definition defines what it means to be a mob.
// Definitions turn into mob Instances.
type Definition struct {
	Id                string
	Aliases           []string
	Name              string
	ShortDescription  string
	DescriptionInRoom string // description when in a room "A giant lizard is here."
	ZoneId            string
	Wandering         rules.WanderDefinition
	MaxHealth         int
	flags             map[Flag]bool
	AC                int
	// Damage is what this mob hits with. The loader always sets it, bare
	// hands when the file says nothing.
	Damage rules.DamageRoll
	// Power is where this mob sits on the scale a player's gear puts them on.
	// The loader always sets it, the bottom of the zone's band when the file
	// says nothing.
	Power int
	// Loot is what might drop into the corpse; see LootEntry.
	Loot []LootEntry
	// Script is the canonical "zone/name" of the Lua this mob runs, empty for
	// none. A name only: the compiled program lives in loader.Content.Scripts,
	// and mobile knows nothing about Lua.
	Script string
}

func NewDefinition(definitionId string,
	name string,
	zoneId string,
	aliases []string,
	shortDescription,
	descriptionInRoom string,
	maxHealth int,
	wandering rules.WanderDefinition,
	AC int,
	aggressive bool) *Definition {
	d := &Definition{
		Id:                definitionId,
		Name:              name,
		Aliases:           aliases,
		ShortDescription:  shortDescription,
		DescriptionInRoom: descriptionInRoom,
		Wandering:         wandering,
		ZoneId:            zoneId,
		flags:             make(map[Flag]bool),
		MaxHealth:         maxHealth,
		AC:                AC,
	}
	if aggressive {
		d.SetFlag(Aggressive)
	}
	return d
}

func (d *Definition) HasAlias(target string) bool {
	for _, a := range d.Aliases {
		if target == a {
			return true
		}
	}
	return false
}

func (d *Definition) SetFlags(flags []Flag) {
	if flags != nil {
		for _, s := range flags {
			d.SetFlag(s)
		}
	}
}

func (d *Definition) SetFlag(flag Flag) {
	d.flags[flag] = true
}

func (d *Definition) HasFlag(flag Flag) bool {
	return d.flags[flag]
}

func (d *Definition) GetFlags() []string {
	var result []string
	for k, v := range d.flags {
		if v {
			result = append(result, string(k))
		}
	}
	return result
}

func (d *Definition) ArmorClass() int {
	// base armor class before modifiers
	return d.AC
}

func (d *Definition) HasResistanceTo(damageType combat.DamageType) bool {
	// TODO resistances
	return false
}

func (d *Definition) IsVulnerableTo(damageType combat.DamageType) bool {
	// TODO vulnerability
	return false
}

func (d *Definition) Matches(target string) bool {
	target = strings.ToLower(target)
	if d.Name == target || d.HasAlias(target) {
		return true
	}
	return false
}
