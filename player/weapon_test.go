package player

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

func TestWeaponDamageRoll(t *testing.T) {
	p := NewTestPlayer(uuid.New(), "dood", nil)
	assert.Equal(t, string(rules.BareHands), p.WeaponDamageRoll(), "nothing wielded")
	knife := object.MakeTestKnife(t)
	knife.Definition.MaxDurability = 1
	knife.Durability = 1
	p.Equipment().Equip(rules.SlotWield, knife)
	assert.Equal(t, "1d8", p.WeaponDamageRoll(), "knife wielded")

	knife.Damage(1)
	require.True(t, knife.Broken())
	assert.Equal(t, string(rules.BareHands), p.WeaponDamageRoll(), "broken knife")
}
