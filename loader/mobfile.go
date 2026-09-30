package loader

import "github.com/watchmud/watchmud/rules"

// mob file might not exist, they are optional
type mobEntry struct {
	Id                  string             `json:"id"`
	Name                string             `json:"name"`
	Aliases             []string           `json:"aliases"`
	ShortDescription    string             `json:"short_description"`
	DescriptionInRoom   string             `json:"description_in_room"`
	WanderingDefinition WanderingEntry     `json:"wandering_definition"`
	Flags               []rules.MobileFlag `json:"flags"`
	MaxHealth           int                `json:"max_health"`

	// AC is the mob's armor class on the same absolute scale a player's is
	// on: rules.BaseArmorClass is unarmored, higher is harder to hit, and
	// combat compares a d20 to it directly.
	//
	// A pointer so that "no ac in the file" is distinguishable from
	// "ac": 0. They are wildly different things -- a missing one means the
	// builder didn't think about it and wants an ordinary target, while a
	// zero means a thing that literally cannot be missed, since a d20 cannot
	// roll below 1. Before this was a pointer every mob that forgot the key
	// silently became the second kind.
	AC         *int `json:"ac"`
	Aggressive bool `json:"aggressive"`
	// Damage is dice notation, "1d4". Absent means bare hands.
	Damage string `json:"damage"`
	// Power is where this mob sits on the same scale as a player's gear.
	// A pointer, like AC: absent means the bottom of the zone's band.
	Power *int `json:"power"`
	// Loot is what might be in the corpse. See loot.go.
	Loot []lootEntry `json:"loot"`
	// Script names the Lua this mob runs: a bare name is this zone's
	// scripts/<name>.lua, "zone/name" any other zone's. See script.go.
	Script string `json:"script"`
}

type WanderingEntry struct {
	CanWander             bool              `json:"can_wander"`
	CheckFrequencySeconds int               `json:"check_frequency_seconds"`
	CheckPercentage       int               `json:"check_percentage"`
	WanderStyle           rules.WanderStyle `json:"wander_style"`
	Path                  []string          `json:"path"`
}
