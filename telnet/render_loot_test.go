package telnet

import (
	"uuid"

	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/world"
)

// a corpse in the start room holding these
func corpseHolding(items ...*object.Instance) func(*world.World, *player.Player, *player.Player) {
	return func(w *world.World, _ *player.Player, _ *player.Player) {
		d := object.NewDefinition("", "the corpse of a rat", "", rules.ObjectCategoryCorpse,
			[]string{"corpse", "rat"}, "the corpse of a rat", "The corpse of a rat is lying here.",
			rules.SlotNone, rules.ArmorTypeNone)
		corpse := object.NewInstance(uuid.New(), d)
		corpse.Contents = object.NewList()
		for _, item := range items {
			_ = corpse.Contents.Add(item)
		}
		_ = w.StartRoom.Inventory.Add(corpse)
	}
}

func poweredKnife(power int) *object.Instance {
	k := testKnife()
	k.Power = power
	return k
}

var lootCases = []commandCase{
	{
		name:      "get from a corpse",
		setup:     corpseHolding(poweredKnife(4), testHelmet()),
		input:     "get knife from corpse",
		want:      "You get knife from the corpse of a rat.\n",
		wantOther: "testdood gets knife from the corpse of a rat.\n",
	},
	{
		name:  "get all from a corpse",
		setup: corpseHolding(poweredKnife(4), testHelmet()),
		input: "get all from corpse",
		want: "You get knife from the corpse of a rat.\n" +
			"You get iron helmet from the corpse of a rat.\n",
		wantOther: "testdood gets knife from the corpse of a rat.\n" +
			"testdood gets iron helmet from the corpse of a rat.\n",
	},
	{
		name:  "get something the corpse doesn't have",
		setup: corpseHolding(poweredKnife(4)),
		input: "get sword from corpse",
		want:  "You don't see that in there.\n",
	},
	{
		name:  "get all from an empty corpse",
		setup: corpseHolding(),
		input: "get all from corpse",
		want:  "There's nothing in there.\n",
	},
	{
		name:  "get from something that isn't here",
		input: "get knife from dragon",
		want:  "You don't see that here.\n",
	},
	{
		// the start room's helmet is right there, and holds nothing
		name:  "get from something that isn't a container",
		input: "get knife from helmet",
		want:  "That's not a container.\n",
	},
	{
		name:  "get from with nothing to get",
		input: "get from corpse",
		want:  "Get what?\n",
	},
	{
		name:  "look in a corpse",
		setup: corpseHolding(poweredKnife(4), testHelmet()),
		input: "look in corpse",
		want: "The corpse of a rat holds:\n" +
			"  knife [power 4]\n" +
			"  iron helmet [power 0]\n",
	},
	{
		name:  "look in an empty corpse",
		setup: corpseHolding(),
		input: "look in corpse",
		want:  "The corpse of a rat is empty.\n",
	},
	{
		name:  "look in something that isn't a container",
		input: "look in helmet",
		want:  "That's not a container.\n",
	},
	{
		name:  "look in nothing",
		input: "look in",
		want:  "Look in what?\n",
	},
}
