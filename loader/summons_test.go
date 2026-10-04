package loader

import (
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/rules"
)

// summonsContent is lootContent's wrathrock and caves, with a mob in each.
func summonsContent(t *testing.T) *Content {
	t.Helper()
	c := lootContent(t)
	c.Zones["caves"].AddMobileDefinition(mobile.NewDefinition("bat", "bat", "caves", nil,
		"a bat", "A bat is here.", 5, rules.WanderDefinition{}, 10, false))
	c.Zones["wrathrock"].AddMobileDefinition(mobile.NewDefinition("guard", "guard", "wrathrock", nil,
		"a guard", "A guard is here.", 20, rules.WanderDefinition{}, 10, false))
	return c
}

// A bare id is the mob's own zone; "zone/id" is anywhere.
func TestMobSummons_resolves(t *testing.T) {
	c := summonsContent(t)
	summons, err := c.mobSummons("caves", mobEntry{Id: "lich", Summons: []string{"bat", "wrathrock/guard"}})
	require.NoError(t, err)
	require.Len(t, summons, 2)
	assert.Same(t, c.Zones["caves"].MobileDefinitions["bat"], summons[0])
	assert.Same(t, c.Zones["wrathrock"].MobileDefinitions["guard"], summons[1])
}

func TestMobSummons_noneIsNone(t *testing.T) {
	summons, err := summonsContent(t).mobSummons("caves", mobEntry{Id: "rat"})
	require.NoError(t, err)
	assert.Empty(t, summons)
}

func TestMobSummons_errors(t *testing.T) {
	c := summonsContent(t)
	for name, ref := range map[string]string{
		"not defined":    "wraith",
		"zone not found": "atlantis/squid",
		"no mob named":   "",
		"zone, no mob":   "caves/",
	} {
		_, err := c.mobSummons("caves", mobEntry{Id: "lich", Summons: []string{ref}})
		assert.ErrorContains(t, err, "caves/lich", name)
	}
}

// Summons name mobs, and mobs load a zone at a time, so a mob may name one in
// a zone that isn't loaded yet: caves loads before wrathrock.
func TestLoadMobileDefinitions_summonsALaterZone(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{
		"caves/mobs.json":     {Data: []byte(`[{"id": "lich", "name": "lich", "max_health": 30, "summons": ["wrathrock/guard"]}]`)},
		"wrathrock/mobs.json": {Data: []byte(`[{"id": "guard", "name": "guard", "max_health": 20}]`)},
	}
	require.NoError(t, c.loadMobileDefinitions(fsys))

	lich := c.Zones["caves"].MobileDefinitions["lich"]
	require.Len(t, lich.Summons, 1)
	assert.Same(t, c.Zones["wrathrock"].MobileDefinitions["guard"], lich.Summons[0])
}

func TestLoadMobileDefinitions_unknownSummonFails(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{
		"caves/mobs.json": {Data: []byte(`[{"id": "lich", "name": "lich", "max_health": 30, "summons": ["wraith"]}]`)},
	}
	assert.ErrorContains(t, c.loadMobileDefinitions(fsys), "caves/lich")
}

// and through a real load: the King calls up barrow skeletons
func TestLoadContent_summons(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	king := c.Zones["barrow"].MobileDefinitions["barrow_king"]
	require.Len(t, king.Summons, 1)
	assert.Same(t, c.Zones["barrow"].MobileDefinitions["barrow_skeleton"], king.Summons[0])
}
