package loader

import (
	"os"
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
