package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The General Store sells what Wrathrock makes, a step up from the kit.
func TestLoadContent_generalStore(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)

	shop := c.Zones["wrathrock"].Shops["general_store"]
	require.NotNil(t, shop)
	require.NotEmpty(t, shop.Stock)
	for _, s := range shop.Stock {
		assert.GreaterOrEqual(t, s.Power, 1, s.Object.Name)
	}
}

// and a satchel to carry it in: a bag, which can be carried
func TestLoadContent_generalStoreSellsABag(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)

	for _, s := range c.Zones["wrathrock"].Shops["general_store"].Stock {
		if spec := s.Object.Container; spec != nil {
			assert.True(t, spec.Portable, s.Object.Name)
			assert.False(t, s.Object.NoTake(), s.Object.Name)
			assert.Positive(t, spec.Capacity, s.Object.Name)
			return
		}
	}
	t.Fatal("no bag for sale")
}
