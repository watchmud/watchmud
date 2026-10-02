package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
