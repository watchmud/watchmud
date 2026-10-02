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
