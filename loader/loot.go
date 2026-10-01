package loader

import (
	"fmt"
	"strings"

	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
)

type lootEntry struct {
	// Object is "zone/id", or a bare id for the mob's own zone.
	Object string `json:"object"`
	// Chance is a percent, 1 to 100.
	Chance int `json:"chance"`
}

// mobLoot resolves a mob's loot table against the object definitions. Every
// zone's objects are loaded before any mobs, so a table can name another
// zone's. Anything that doesn't resolve fails startup: a typo in a loot table
// would otherwise be a drop that silently never happens.
func (c *Content) mobLoot(zoneName string, mob mobEntry) ([]mobile.LootEntry, error) {
	var loot []mobile.LootEntry
	for _, l := range mob.Loot {
		defn, err := c.objectRef(zoneName, l.Object)
		if err != nil {
			return nil, fmt.Errorf("mob %s/%s: loot: %w", zoneName, mob.Id, err)
		}
		if l.Chance < 1 || l.Chance > 100 {
			return nil, fmt.Errorf("mob %s/%s: loot %q: chance %d is not 1-100", zoneName, mob.Id, l.Object, l.Chance)
		}
		loot = append(loot, mobile.LootEntry{Object: defn, Chance: l.Chance})
	}
	return loot, nil
}

// objectRef finds the object a piece of content names: "zone/id", or a bare
// id for the naming zone's own. Every zone's objects are loaded before
// anything that names one, so it can be any zone's.
func (c *Content) objectRef(zoneName, ref string) (*object.Definition, error) {
	zoneId, objectId := zoneName, ref
	if z, o, found := strings.Cut(ref, "/"); found {
		zoneId, objectId = z, o
	}
	if objectId == "" {
		return nil, fmt.Errorf("names no object")
	}
	zone, ok := c.Zones[zoneId]
	if !ok {
		return nil, fmt.Errorf("%q: zone not found", ref)
	}
	defn, ok := zone.ObjectDefinitions[objectId]
	if !ok {
		return nil, fmt.Errorf("%q: object not defined", ref)
	}
	return defn, nil
}
