package rules

// StartingGear is what a brand-new character is handed at creation, in the
// order content declared it: content/rules/starting_gear.json.
//
// It is a rule rather than a zone instruction because it is a statement about
// characters, not about a room -- nothing is ever placed anywhere, the items
// are created in the new player's hands. The items themselves are ordinary
// object definitions in an ordinary zone, so a builder edits the kit the same
// way they edit anything else.
//
// Nothing here says what role the kit adds up to. It can't: a role is read off
// the equipment at the moment it is asked for, so the kit decides what a level
// 1 character looks like on their first prompt and stops mattering the moment
// they wear something else.
type StartingGear []StartingGearItem

// A StartingGearItem is one object definition, named by the zone that defines
// it, and whether the character starts with it worn.
//
// Equip is a bool rather than a slot because object.Definition already names
// the one slot the item goes in: repeating it here would be a second number to
// keep in step with the first, and the loader would only be checking that they
// agree. An item with Equip set that isn't wearable is a content error, caught
// at startup.
type StartingGearItem struct {
	ZoneId       string `json:"zone"`
	DefinitionId string `json:"object"`
	Equip        bool   `json:"equip"`
	// Power is what the item is made at. A plain int, not a pointer: absent
	// and zero agree, as they do in a save record. Set it to the bottom of
	// the newbie zone's band so a new character starts level with the
	// weakest thing out there, not below it.
	Power int `json:"power"`
	// Backfill hands the item, once, to a character created before it was
	// in the kit: at their next login (World.Arrive), worn if its slot is
	// free. The record's Backfilled list is what makes it once.
	Backfill bool `json:"backfill"`
}
