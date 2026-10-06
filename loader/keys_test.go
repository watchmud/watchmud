package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/spaces"
)

// Every lock in the real content has a key a player can come by -- a reset
// puts one down (and not only inside what it locks), a mob placed somewhere
// drops one, or a shop sells one -- so no door or chest is shut for good.
func TestKeys_everyLockHasAKeyToBeHad(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)

	// where each key, by "zone/id", can be had; and inside which container
	// definition ("zone/id") a reset puts it, if that's the only place
	obtainable := map[string]bool{}
	insideOnly := map[string][]string{}
	placed := map[string]bool{} // mobs a reset puts down
	for zoneId, z := range c.Zones {
		for _, cmd := range z.Commands {
			switch cmd := cmd.(type) {
			case spaces.CreateObject:
				zid := cmd.ZoneId
				if zid == "" {
					zid = zoneId
				}
				ref := zid + "/" + cmd.ObjectDefinitionId
				if cmd.ContainerId == "" {
					obtainable[ref] = true
				} else {
					insideOnly[ref] = append(insideOnly[ref], zoneId+"/"+cmd.ContainerId)
				}
			case spaces.CreateMobile:
				zid := cmd.ZoneId
				if zid == "" {
					zid = zoneId
				}
				placed[zid+"/"+cmd.MobileDefinitionId] = true
			}
		}
		for _, shop := range z.Shops {
			for _, item := range shop.Stock {
				obtainable[item.Object.ObjectId.Ref()] = true
			}
		}
	}
	for zoneId, z := range c.Zones {
		for id, mob := range z.MobileDefinitions {
			if !placed[zoneId+"/"+id] {
				continue
			}
			for _, l := range mob.Loot {
				if l.Chance > 0 {
					obtainable[l.Object.ObjectId.Ref()] = true
				}
			}
		}
	}

	// a key only ever inside a container is to be had if that container
	// isn't the one it locks
	reachable := func(key, locks string) bool {
		if obtainable[key] {
			return true
		}
		for _, in := range insideOnly[key] {
			if in != locks {
				return true
			}
		}
		return false
	}

	checked := 0
	for zoneId, z := range c.Zones {
		for _, d := range z.Doors {
			if d.Key != "" {
				checked++
				assert.True(t, reachable(d.Key, ""), "door %q in %s: nothing gives its key %s", d.Name, zoneId, d.Key)
			}
		}
		for id, def := range z.ObjectDefinitions {
			if def.Container != nil && def.Container.Key != "" {
				checked++
				assert.True(t, reachable(def.Container.Key, zoneId+"/"+id),
					"chest %s/%s: nothing gives its key %s outside itself", zoneId, id, def.Container.Key)
			}
		}
	}
	assert.Positive(t, checked, "the content has locks to check")
}
