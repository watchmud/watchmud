package loader

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const quietScript = `function on_fight_start(me, foe) me:say("hi") end`

// lootContent (loot_test.go) is wrathrock plus a "caves" zone.
func TestMobScript_ownZone(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{"caves/scripts/ghoul.lua": {Data: []byte(quietScript)}}

	ref, err := c.mobScript(fsys, "caves", mobEntry{Id: "ghoul", Script: "ghoul"})

	require.NoError(t, err)
	assert.Equal(t, "caves/ghoul", ref)
	require.Contains(t, c.Scripts, "caves/ghoul")
	assert.Equal(t, "caves/ghoul", c.Scripts["caves/ghoul"].Name())
}

func TestMobScript_otherZone(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{"wrathrock/scripts/shout.lua": {Data: []byte(quietScript)}}

	ref, err := c.mobScript(fsys, "caves", mobEntry{Id: "ghoul", Script: "wrathrock/shout"})

	require.NoError(t, err)
	assert.Equal(t, "wrathrock/shout", ref)
}

func TestMobScript_noneIsNone(t *testing.T) {
	c := lootContent(t)
	ref, err := c.mobScript(fstest.MapFS{}, "caves", mobEntry{Id: "bat"})
	require.NoError(t, err)
	assert.Empty(t, ref)
	assert.Empty(t, c.Scripts)
}

// Two mobs naming one script compile it once.
func TestMobScript_sharedCompilesOnce(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{"caves/scripts/ghoul.lua": {Data: []byte(quietScript)}}
	_, err := c.mobScript(fsys, "caves", mobEntry{Id: "a", Script: "ghoul"})
	require.NoError(t, err)
	first := c.Scripts["caves/ghoul"]

	_, err = c.mobScript(fsys, "caves", mobEntry{Id: "b", Script: "ghoul"})
	require.NoError(t, err)
	assert.Same(t, first, c.Scripts["caves/ghoul"])
}

func TestMobScript_errors(t *testing.T) {
	fsys := fstest.MapFS{
		"caves/scripts/broken.lua": {Data: []byte(`function on_fight_start(`)},
		"caves/scripts/typo.lua":   {Data: []byte(`function on_fightstart() end`)},
	}
	for _, tc := range []struct{ script, want string }{
		{"missing", "caves/scripts/missing.lua"},
		{"nowhere/ghoul", "zone not found"},
		{"caves/", "names no script"},
		{"broken", "caves/broken"},
		{"typo", "on_fightstart"},
	} {
		t.Run(tc.script, func(t *testing.T) {
			_, err := lootContent(t).mobScript(fsys, "caves", mobEntry{Id: "ghoul", Script: tc.script})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "caves/ghoul", "names the mob")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
