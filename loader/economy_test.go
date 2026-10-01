package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/rules"
)

// The real game has an economy; a content set without economy.json loads with
// it switched off.
func TestLoadContent_economy(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	e := c.Catalog.Economy
	assert.Positive(t, e.CoinsPerPower)
	assert.Positive(t, e.Price(rules.ObjectCategoryArmor, 1))
	assert.Less(t, e.SellPercent, 100)

	cat, err := LoadCatalog(os.DirFS("../testcontent/rules"))
	require.NoError(t, err)
	assert.NoError(t, cat.Economy.Check())
}
