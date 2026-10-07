package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/rules"
)

// The real game has heal, and so does the content the world tests load.
func TestLoadCatalog_abilities(t *testing.T) {
	for _, dir := range []string{"../content/rules", "../testcontent/rules"} {
		cat, err := LoadCatalog(os.DirFS(dir))
		require.NoError(t, err, dir)
		heal, found := cat.Abilities["heal"]
		require.True(t, found, dir)
		assert.Positive(t, heal.Mana, dir)
		assert.Positive(t, heal.Cooldown, dir)
	}
}

func TestObjectAbilities(t *testing.T) {
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)
	require.NoError(t, cat.SetAbilities([]*rules.Ability{{Id: "heal", Name: "heal", Target: rules.TargetFriend}}))

	got, err := objectAbilities("hollowfield", objectEntry{Id: "censer", Abilities: []string{"heal"}}, cat)
	require.NoError(t, err)
	assert.Equal(t, []string{"heal"}, got)

	_, err = objectAbilities("hollowfield", objectEntry{Id: "censer", Abilities: []string{"hael"}}, cat)
	assert.ErrorContains(t, err, "hollowfield/censer")
}

// a potion is drunk by its drinker: an ability aimed at a foe isn't one
func TestObjectQuaff(t *testing.T) {
	cat, err := rules.NewTestCatalog()
	require.NoError(t, err)
	require.NoError(t, cat.SetAbilities([]*rules.Ability{
		{Id: "heal", Name: "heal", Target: rules.TargetFriend},
		{Id: "smite", Name: "smite", Target: rules.TargetFoe},
	}))

	got, err := objectQuaff("wrathrock", objectEntry{Id: "draught", Quaff: "heal"}, cat)
	require.NoError(t, err)
	assert.Equal(t, "heal", got)

	_, err = objectQuaff("wrathrock", objectEntry{Id: "draught", Quaff: "hael"}, cat)
	assert.ErrorContains(t, err, "unknown ability")
	_, err = objectQuaff("wrathrock", objectEntry{Id: "draught", Quaff: "smite"}, cat)
	assert.ErrorContains(t, err, "isn't something to drink")
}

// the General Store sells one
func TestLoadContent_healingDraught(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	d, found := c.Zones["wrathrock"].ObjectDefinitions["healing_draught"]
	require.True(t, found)
	assert.Equal(t, "heal", d.Quaff)
}

// a newbie's way to a heal, and a barrow healer's better one
func TestLoadContent_healersHeal(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	for _, ref := range [][2]string{
		{"hollowfield", "sprig_censer"},
		{"barrow", "bone_charm"},
	} {
		d, found := c.Zones[ref[0]].ObjectDefinitions[ref[1]]
		require.True(t, found, ref)
		assert.Contains(t, d.Abilities, "heal", ref)
	}
}

// smite comes from weapons: a bandit's cudgel in the Hollowfields, and the
// barrow's blade for a better one
func TestLoadContent_weaponsSmite(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	smite, found := c.Catalog.Abilities["smite"]
	require.True(t, found)
	assert.Equal(t, "foe", string(smite.Target))
	for _, ref := range [][2]string{
		{"hollowfield", "bandit_cudgel"},
		{"barrow", "barrow_blade"},
	} {
		d, found := c.Zones[ref[0]].ObjectDefinitions[ref[1]]
		require.True(t, found, ref)
		assert.Contains(t, d.Abilities, "smite", ref)
	}
}
