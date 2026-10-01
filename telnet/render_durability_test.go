package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/rules"
)

// What a player sees of durability: the condition of each piece in the
// listing, and one line when something gives out.
func TestRenderEquipment_condition(t *testing.T) {
	got := render(event.Equipment{Items: []event.EquippedItem{
		{Slot: rules.SlotWield, Id: "id-1", ShortDescription: "a knife", Durability: 17, MaxDurability: 25},
		{Slot: rules.SlotBody, Id: "id-2", ShortDescription: "a chain shirt", Durability: 0, MaxDurability: 80, Broken: true},
		{Slot: rules.SlotHead, Id: "id-3", ShortDescription: "an heirloom helm"},
	}}, "testdood")

	assert.Equal(t,
		"You are using (power 0):\n"+
			"wield\ta knife\t[power 0] (17/25) (id-1)\n"+
			"body\ta chain shirt\t[power 0] (broken) (id-2)\n"+
			"head\tan heirloom helm\t[power 0] (id-3)\n\n",
		got)
}

func TestRenderBroke(t *testing.T) {
	broke := event.Broke{Actor: "testdood", Item: "chain shirt"}

	assert.Equal(t, "Your chain shirt gives out, ruined.\n", plain(render(broke, "testdood")))
	assert.Equal(t, "testdood's chain shirt gives out, ruined.\n", render(broke, "otherdood"))
}

func TestRenderGearDamaged(t *testing.T) {
	assert.Equal(t, "Dying has taken its toll on your equipment.\n",
		render(event.GearDamaged{Items: 1}, "testdood"))
	assert.Equal(t, "Dying has taken its toll on your equipment (3 pieces).\n",
		render(event.GearDamaged{Items: 3}, "testdood"))
}

func TestRenderRepaired(t *testing.T) {
	assert.Equal(t, "The smith works on a chain shirt until it's as good as new.\n",
		render(event.Repaired{Item: "a chain shirt"}, "testdood"), "no economy: free")
	assert.Equal(t, "The smith works on a chain shirt until it's as good as new, for 12 coins.\n",
		render(event.Repaired{Item: "a chain shirt", Cost: 12}, "testdood"))
	assert.Equal(t, "A chain shirt would cost 12 coins, and you have 1 coin.\n",
		render(event.TooExpensive{Item: "a chain shirt", Cost: 12, Coins: 1}, "testdood"))
}
