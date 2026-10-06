package telnet

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/rules"
)

func TestRenderEquipment_power(t *testing.T) {
	got := render(event.Equipment{Power: 6, Items: []event.EquippedItem{
		{Slot: rules.SlotWield, Id: "id-1", ShortDescription: "a knife", Power: 4},
		{Slot: rules.SlotHead, Id: "id-2", ShortDescription: "a helm", Power: 8},
	}}, "testdood")

	assert.Equal(t,
		"Equipment (power 6)\n"+
			"  wielded  a knife  power 4\n"+
			"  head     a helm   power 8\n",
		plain(got))
}

// Every step of consider, from the far bottom of the clamp to the top. Delta
// is yours less theirs, so negative is trouble.
func TestRenderConsidered(t *testing.T) {
	cases := []struct {
		delta int
		want  string
	}{
		{-10, "Barrow-King would kill you without noticing."},
		{-5, "Barrow-King would probably kill you."},
		{-4, "Barrow-King would be a real challenge."},
		{-2, "Barrow-King would be a real challenge."},
		{-1, "Barrow-King looks like a fair fight."},
		{1, "Barrow-King looks like a fair fight."},
		{2, "Barrow-King should be easy."},
		{4, "Barrow-King should be easy."},
		{5, "You could kill Barrow-King with your eyes closed."},
		{10, "You could kill Barrow-King with your eyes closed."},
	}
	for _, c := range cases {
		got := render(event.Considered{Target: "Barrow-King", TargetPower: 15, YourPower: 15 + c.delta, Delta: c.delta}, "testdood")
		assert.Equal(t, c.want+" (power 15; you are "+itoa(15+c.delta)+")\n", got, "delta %d", c.delta)
	}
}

// Mob names are lower case ("field rat"), and here one starts the sentence.
func TestRenderConsidered_capitalizes(t *testing.T) {
	got := render(event.Considered{Target: "field rat", TargetPower: 1, YourPower: 0, Delta: -1}, "testdood")
	assert.Equal(t, "Field rat looks like a fair fight. (power 1; you are 0)\n", got)
}

func itoa(i int) string { return fmt.Sprint(i) }

func TestRenderDecayed(t *testing.T) {
	assert.Equal(t, "The corpse of a rat crumbles to dust.\n",
		render(event.Decayed{Item: "the corpse of a rat"}, "testdood"))
}
