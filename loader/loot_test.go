package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
	"github.com/watchmud/watchmud/zonereset"
)

// lootContent is contentWithGear's wrathrock plus a second zone, so a loot
// table can reach across.
func lootContent(t *testing.T) *Content {
	t.Helper()
	c := contentWithGear(t, nil)
	caves := spaces.NewZone("caves", "Caves", zonereset.NEVER, 0)
	caves.AddObjectDefinition(object.NewDefinition("bone", "bone", "caves",
		rules.ObjectCategoryOther, nil, "a bone",
		"A bone is here.", rules.SlotNone, rules.ArmorTypeNone, []rules.ObjectBehavior{}))
	c.addZone(caves)
	return c
}

// A bare id is the mob's own zone; "zone/id" is anywhere.
func TestMobLoot_resolvesObjects(t *testing.T) {
	c := lootContent(t)
	loot, err := c.mobLoot("caves", mobEntry{Id: "ghoul", Loot: []lootEntry{
		{Object: "bone", Chance: 40},
		{Object: "wrathrock/knife", Chance: 5},
	}})
	require.NoError(t, err)
	require.Len(t, loot, 2)
	assert.Equal(t, "bone", loot[0].Object.ObjectId.DefinitionId)
	assert.Equal(t, "caves", loot[0].Object.ObjectId.ZoneId)
	assert.Equal(t, 40, loot[0].Chance)
	assert.Equal(t, "knife", loot[1].Object.ObjectId.DefinitionId)
	assert.Equal(t, "wrathrock", loot[1].Object.ObjectId.ZoneId)
}

func TestMobLoot_noneIsNone(t *testing.T) {
	loot, err := lootContent(t).mobLoot("caves", mobEntry{Id: "bat"})
	require.NoError(t, err)
	assert.Empty(t, loot)
}

func TestMobLoot_errors(t *testing.T) {
	c := lootContent(t)
	cases := map[string]lootEntry{
		"not defined":     {Object: "skull", Chance: 10},
		"zone not found":  {Object: "atlantis/trident", Chance: 10},
		"chance 0":        {Object: "bone", Chance: 0},
		"chance 101":      {Object: "bone", Chance: 101},
		"no object named": {Chance: 10},
	}
	for name, entry := range cases {
		_, err := c.mobLoot("caves", mobEntry{Id: "ghoul", Loot: []lootEntry{entry}})
		assert.ErrorContains(t, err, "caves/ghoul", name)
	}
}

// and through a real load: the King drops his blade
func TestLoadContent_loot(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)

	king := c.Zones["barrow"].MobileDefinitions["barrow_king"]
	var drops []string
	for _, l := range king.Loot {
		drops = append(drops, l.Object.ObjectId.DefinitionId)
	}
	assert.Contains(t, drops, "barrow_blade")
}
