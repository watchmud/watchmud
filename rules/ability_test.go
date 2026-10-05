package rules

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbility_unmarshal(t *testing.T) {
	var a Ability
	require.NoError(t, json.Unmarshal([]byte(`{
	    "id": "heal",
    	"name": "heal",
	    "mana": 20,
    	"cooldown": "10s",
	    "target": "friend",
	    "amount" : {
    	  "base": 10,
	      "per_power": 2
	    }
	  }`), &a))
	assert.Equal(t, Ability{
		Id:       "heal",
		Name:     "heal",
		Mana:     20,
		Cooldown: 10 * time.Second,
		Target:   TargetFriend,
		Amount: AbilityAmount{
			Base:     10,
			PerPower: 2,
		},
	}, a)
}

func TestAbility_unmarshalBadCooldown(t *testing.T) {
	var a Ability
	assert.Error(t, json.Unmarshal([]byte(`{"id":"heal", "cooldown":"soon"}`), &a))
}

func TestAbility_noCooldownIsZero(t *testing.T) {
	var a Ability
	require.NoError(t, json.Unmarshal([]byte(`{"id": "x", "name": "x", "target": "self"}`), &a))
	assert.Zero(t, a.Cooldown)
}

func TestAbilityAmount_for(t *testing.T) {
	amt := AbilityAmount{Base: 10, PerPower: 2}
	assert.Equal(t, 10, amt.For(0))
	assert.Equal(t, 12, amt.For(1))
	assert.Equal(t, 26, amt.For(8))
}

func testHeal() *Ability {
	return &Ability{Id: "heal", Name: "heal", Mana: 20, Cooldown: 10 * time.Second,
		Target: TargetFriend, Amount: AbilityAmount{Base: 10, PerPower: 2}}
}

func TestCatalog_setAbilities(t *testing.T) {
	c, err := NewTestCatalog()
	require.NoError(t, err)
	zap := &Ability{Id: "zap", Name: "zap", Target: TargetFoe}
	require.NoError(t, c.SetAbilities([]*Ability{testHeal(), zap}))

	assert.Equal(t, "heal", c.Abilities["heal"].Id)
	assert.Equal(t, []*Ability{c.Abilities["heal"], zap}, c.AbilityList(), "declaration order")
}

func TestCatalog_setAbilitiesRefuses(t *testing.T) {
	for name, a := range map[string]*Ability{
		"no id":             {Name: "heal", Target: TargetSelf},
		"no name":           {Id: "heal", Target: TargetSelf},
		"negative mana":     {Id: "heal", Name: "heal", Target: TargetSelf, Mana: -1},
		"negative cooldown": {Id: "heal", Name: "heal", Target: TargetSelf, Cooldown: -time.Second},
		"unknown target":    {Id: "heal", Name: "heal", Target: "everyone"},
		"no target":         {Id: "heal", Name: "heal"},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewTestCatalog()
			require.NoError(t, err)
			assert.Error(t, c.SetAbilities([]*Ability{a}))
		})
	}
}

func TestCatalog_setAbilitiesRefusesDuplicates(t *testing.T) {
	c, err := NewTestCatalog()
	require.NoError(t, err)
	assert.Error(t, c.SetAbilities([]*Ability{testHeal(), testHeal()}))
}

// recall's two flags: refused mid-fight, and a wizard's without gear or cooldown
func TestAbility_flags(t *testing.T) {
	var a Ability
	require.NoError(t, json.Unmarshal([]byte(`{"id": "recall", "name": "recall", "target": "none",
		"cooldown": "60s", "not_in_fight": true, "wizards": true}`), &a))
	assert.True(t, a.NotInFight)
	assert.True(t, a.Wizards)
	assert.Equal(t, TargetNone, a.Target)
}
