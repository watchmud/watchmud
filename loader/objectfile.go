package loader

import "github.com/watchmud/watchmud/rules"

// object file is optional
type objectEntry struct {
	Id                  string                 `json:"id"`
	Name                string                 `json:"name"`
	Category            rules.ObjectCategory   `json:"category"`
	Aliases             []string               `json:"aliases"`
	ShortDescription    string                 `json:"short_description"`
	DescriptionOnGround string                 `json:"description_on_ground"`
	EquipmentSlot       rules.EquipmentSlot    `json:"equipment_slot"` // TODO bad name
	Behaviors           []rules.ObjectBehavior `json:"behaviors"`
	ArmorType           rules.ArmorType        `json:"armor_type"`

	// Durability overrides what the durability table would give this object,
	// for the one blade in the game that deserves it. A pointer so that
	// "says nothing" (take the table's number) is distinguishable from
	// "durability": 0, which is a thing that never wears out.
	Durability *int `json:"durability"`

	// Damage is dice notation, "1d6". Required on anything with
	// "equipment_slot": "wield", refused on anything else.
	Damage string `json:"damage"`

	// Roles is what this object contributes to each role while equipped,
	// keyed on rules.Role.Id: {"tank": 3}. Optional, and meaningless on
	// anything without a wear_location.
	Roles map[string]int `json:"roles"`

	// Abilities this grants while equipped, by rules.Ability id: ["heal"]
	Abilities []string `json:"abilities"`

	// Quaff makes it a potion: the ability drinking it uses, on the drinker.
	Quaff string `json:"quaff"`

	// Container makes it a chest (chests.go).
	Container *containerEntry `json:"container"`
}

// containerEntry is a chest's lid: how it starts, and the key that fits.
type containerEntry struct {
	Closed bool   `json:"closed"`
	Locked bool   `json:"locked"`
	Key    string `json:"key"`
	// Portable makes it a bag: carried, no lid, saved with its carrier.
	Portable bool `json:"portable"`
	// Capacity is how many things fit; zero, no limit.
	Capacity int `json:"capacity"`
}
