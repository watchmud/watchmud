package loader

import (
	"fmt"
	"strings"

	"github.com/watchmud/watchmud/mobile"
)

// mobSummons resolves what a mob may summon against the mob definitions.
// Anything that doesn't resolve fails startup: a typo would otherwise be a
// summon that fails every time the script reaches for it, in the middle of
// the fight it was written for.
func (c *Content) mobSummons(zoneName string, mob mobEntry) ([]*mobile.Definition, error) {
	var summons []*mobile.Definition
	for _, ref := range mob.Summons {
		defn, err := c.mobileRef(zoneName, ref)
		if err != nil {
			return nil, fmt.Errorf("mob %s/%s: summons: %w", zoneName, mob.Id, err)
		}
		summons = append(summons, defn)
	}
	return summons, nil
}

// mobileRef finds the mob a piece of content names: "zone/id" or a bare id
// for the naming zone's own. objectRef's twin -- but mobs load a zone at a time,
// so only ask once every zone's are in.
func (c *Content) mobileRef(zoneName, ref string) (*mobile.Definition, error) {
	zoneId, mobId := zoneName, ref
	if z, m, found := strings.Cut(ref, "/"); found {
		zoneId, mobId = z, m
	}
	if mobId == "" {
		return nil, fmt.Errorf("names no mob")
	}
	zone, ok := c.Zones[zoneId]
	if !ok {
		return nil, fmt.Errorf("%q: zone not found", ref)
	}
	defn, ok := zone.MobileDefinitions[mobId]
	if !ok {
		return nil, fmt.Errorf("%q: mob not defined", ref)
	}
	return defn, nil
}
