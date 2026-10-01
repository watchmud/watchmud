package loader

import (
	"fmt"
	"io/fs"
	"path"

	"github.com/watchmud/watchmud/spaces"
)

// shops.json is optional, per zone: which of its rooms trade, and what each
// sells. Objects are named the way loot tables name them.
type shopEntry struct {
	Room  string `json:"room"`
	Sells []struct {
		Object string `json:"object"`
		Power  int    `json:"power"`
	} `json:"sells"`
}

// loadShops runs after the objects, which a shop's stock names. A shop in a
// room that doesn't exist, two in one room, or stock that doesn't resolve,
// fails startup: a typo here would be a store that silently isn't one.
func (c *Content) loadShops(fsys fs.FS) error {
	for _, zonename := range c.zoneNames() {
		entries, err := readOptionalJSONFile[[]shopEntry](fsys, path.Join(zonename, "shops.json"))
		if err != nil {
			return err
		}
		zone := c.Zones[zonename]
		for _, e := range entries {
			if _, ok := zone.Rooms[e.Room]; !ok {
				return fmt.Errorf("shop %s/%s: room not found", zonename, e.Room)
			}
			if _, dup := zone.Shops[e.Room]; dup {
				return fmt.Errorf("shop %s/%s: a room has one shop", zonename, e.Room)
			}
			shop := &spaces.Shop{}
			for _, s := range e.Sells {
				defn, err := c.objectRef(zonename, s.Object)
				if err != nil {
					return fmt.Errorf("shop %s/%s: %w", zonename, e.Room, err)
				}
				if s.Power < 0 {
					return fmt.Errorf("shop %s/%s: %q at power %d", zonename, e.Room, s.Object, s.Power)
				}
				shop.Stock = append(shop.Stock, spaces.ShopItem{Object: defn, Power: s.Power})
			}
			zone.Shops[e.Room] = shop
		}
	}
	return nil
}
