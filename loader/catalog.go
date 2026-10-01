package loader

import (
	"io/fs"

	"github.com/watchmud/watchmud/rules"
)

func LoadCatalog(rulesFS fs.FS) (*rules.Catalog, error) {
	species, err := readJSONFile[[]*rules.Species](rulesFS, "species.json")
	if err != nil {
		return nil, err
	}

	roles, err := readJSONFile[[]*rules.Role](rulesFS, "roles.json")
	if err != nil {
		return nil, err
	}

	armor, err := readJSONFile[rules.ArmorTypeContent](rulesFS, "armor.json")
	if err != nil {
		return nil, err
	}

	mudTime, err := readJSONFile[rules.MudTime](rulesFS, "mudtime.json")
	if err != nil {
		return nil, err
	}

	c, err := rules.NewCatalog(mudTime, species, roles, armor)
	if err != nil {
		return nil, err
	}

	// Optional, and an absent one is not an empty table with a zero default:
	// it is durability switched off, since every piece of gear then has a max
	// of rules.Indestructible. See DurabilityTable.
	durability, err := readOptionalJSONFile[rules.DurabilityTable](rulesFS, "durability.json")
	if err != nil {
		return nil, err
	}
	c.Durability = durability

	// Optional too, and absent is the economy switched off: see rules.Economy.
	economy, err := readOptionalJSONFile[rules.Economy](rulesFS, "economy.json")
	if err != nil {
		return nil, err
	}
	if err := economy.Check(); err != nil {
		return nil, err
	}
	c.Economy = economy

	// Optional: a content set with no starting_gear.json hands new characters
	// nothing, which is a game you can play. What it can't be is wrong, so
	// every item in it is checked against the zones once they are loaded --
	// see Content.checkStartingGear.
	gear, err := readOptionalJSONFile[rules.StartingGear](rulesFS, "starting_gear.json")
	if err != nil {
		return nil, err
	}
	c.StartingGear = gear

	return c, nil
}
