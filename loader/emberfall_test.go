package loader

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

func emberfall(t *testing.T) (*Content, *spaces.Zone) {
	t.Helper()
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	z, ok := c.Zones["emberfall"]
	require.True(t, ok, "no emberfall zone")
	return c, z
}

// The ruins are east of the Bandit Camp, and the way back is west.
func TestEmberfall_joinsAtTheCamp(t *testing.T) {
	c, _ := emberfall(t)
	camp := c.Zones["hollowfield"].Rooms["bandit_camp"]
	road := camp.DestinationRoom(rules.DirectionEast)
	require.NotNil(t, road, "nothing east of the camp")
	assert.Equal(t, "emberfall", road.Zone.Id)
	assert.Equal(t, camp, road.DestinationRoom(rules.DirectionWest))
}

// The tower stays shut until something opens it: a locked door whose key is
// defined and nothing gives.
func TestEmberfall_theTowerIsSealed(t *testing.T) {
	c, z := emberfall(t)
	foot := z.Rooms["tower_door"]
	require.NotNil(t, foot)
	door := foot.DoorTo(rules.DirectionEast)
	require.NotNil(t, door, "no door into the tower")
	assert.Equal(t, z.Rooms["tower"], foot.DestinationRoom(rules.DirectionEast))
	assert.True(t, door.Closed && door.Locked, "the tower door is shut and locked")
	assert.Equal(t, "emberfall/tower_key", door.Key)
	require.NotNil(t, z.ObjectDefinitions["tower_key"])

	for zoneId, zone := range c.Zones {
		for _, cmd := range zone.Commands {
			if co, ok := cmd.(spaces.CreateObject); ok {
				assert.NotEqual(t, "tower_key", co.ObjectDefinitionId, "%s puts the tower key down", zoneId)
			}
		}
		for id, mob := range zone.MobileDefinitions {
			for _, l := range mob.Loot {
				assert.NotEqual(t, "emberfall/tower_key", l.Object.ObjectId.Ref(), "%s/%s drops the tower key", zoneId, id)
			}
		}
	}
}

// The biggest place in the game, with sewers beneath it.
func TestEmberfall_size(t *testing.T) {
	_, z := emberfall(t)
	assert.GreaterOrEqual(t, len(z.Rooms), 30)
	assert.LessOrEqual(t, len(z.Rooms), 40)
	down := false
	for _, r := range z.Rooms {
		if d := r.DestinationRoom(rules.DirectionDown); d != nil && d.Zone == z {
			down = true
		}
	}
	assert.True(t, down, "no way down into the sewers")
}

// Every monster is in the zone's band, and the way in is quiet: nothing
// aggressive is put on the road, and nothing aggressive wanders, so nothing
// aggressive can come to it.
func TestEmberfall_mobsInBand(t *testing.T) {
	_, z := emberfall(t)
	require.NotEmpty(t, z.MobileDefinitions)
	for id, mob := range z.MobileDefinitions {
		assert.GreaterOrEqual(t, mob.Power, 4, id)
		assert.LessOrEqual(t, mob.Power, 5, id)
		if mob.HasFlag(rules.MobileFlagAggressive) {
			assert.False(t, mob.Wandering.CanWander, "%s is aggressive and wanders", id)
		}
	}
	placed := 0
	for _, cmd := range z.Commands {
		cm, ok := cmd.(spaces.CreateMobile)
		if !ok {
			continue
		}
		placed++
		mob := z.MobileDefinitions[cm.MobileDefinitionId]
		require.NotNil(t, mob, cm.MobileDefinitionId)
		if strings.HasPrefix(cm.RoomId, "road_") || cm.RoomId == "town_gate" {
			assert.False(t, mob.HasFlag(rules.MobileFlagAggressive), "%s is aggressive on the road, in %s", cm.MobileDefinitionId, cm.RoomId)
		}
	}
	assert.Positive(t, placed, "a reset puts the monsters down")
}

// Nothing here is a boss's own until the tower opens: everything that drops
// can be sold and donated. Components are plain things, worn by nobody.
func TestEmberfall_lootIsTradeable(t *testing.T) {
	_, z := emberfall(t)
	components := 0
	for id, def := range z.ObjectDefinitions {
		if id == "tower_key" {
			continue // a key, which nothing gives
		}
		assert.False(t, slices.Contains(def.Behaviors, rules.ObjectBehaviorNoSell), "%s is noSell", id)
		assert.False(t, slices.Contains(def.Behaviors, rules.ObjectBehaviorNoDonate), "%s is noDonate", id)
		if def.ObjectCategory == rules.ObjectCategoryOther {
			components++
			assert.Equal(t, rules.SlotNone, def.EquipmentSlot, "%s is a component and wearable", id)
		}
	}
	assert.GreaterOrEqual(t, components, 4, "orc tusk, daemon horn, ichor, wyvern scale")
}

// The guild's gear: things that let you cast, dropped by what lives here.
func TestEmberfall_artifactsGrant(t *testing.T) {
	_, z := emberfall(t)
	dropped := map[string]bool{}
	for _, mob := range z.MobileDefinitions {
		for _, l := range mob.Loot {
			dropped[l.Object.ObjectId.Ref()] = true
		}
	}
	artifacts := 0
	for id, def := range z.ObjectDefinitions {
		if len(def.Abilities) == 0 {
			continue
		}
		artifacts++
		assert.True(t, dropped[def.ObjectId.Ref()], "%s grants %v and nothing drops it", id, def.Abilities)
	}
	assert.GreaterOrEqual(t, artifacts, 4)
}
