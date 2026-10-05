package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/rules"
)

// The temple token, as the real content has it: worn on the neck, grants
// recall, never wears out -- and every character is handed one, worn, and
// backfilled to those who came before it.
func TestLoadContent_templeToken(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)

	token := c.Zones["wrathrock"].ObjectDefinitions["temple_token"]
	require.NotNil(t, token)
	assert.Equal(t, rules.EquipmentSlot("neck"), token.EquipmentSlot)
	assert.Equal(t, []string{"recall"}, token.Abilities)
	assert.Equal(t, rules.Indestructible, token.MaxDurability)

	recall := c.Catalog.Abilities["recall"]
	require.NotNil(t, recall)
	assert.Equal(t, rules.TargetNone, recall.Target)
	assert.True(t, recall.NotInFight)
	assert.True(t, recall.Wizards)

	var kit *rules.StartingGearItem
	for i, item := range c.Catalog.StartingGear {
		if item.DefinitionId == "temple_token" {
			kit = &c.Catalog.StartingGear[i]
		}
	}
	require.NotNil(t, kit, "in the starting gear")
	assert.True(t, kit.Equip)
	assert.True(t, kit.Backfill)
}
