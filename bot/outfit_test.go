package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// What the equipment list looks like with color off: a padded table under its
// heading, a slot of one or two words, condition last or not at all.
const starterEquipment = "Equipment (power 1)\n" +
	"  wielded     a training dagger  power 1  25/25\n" +
	"  body        a coarse wool tunic  power 1  20/20\n" +
	"  about body  a travelers cloak  power 1  6/20\n" +
	"  neck        a temple token     power 1\n"

func TestReadEquipment(t *testing.T) {
	rows := readEquipment(starterEquipment)
	require.Len(t, rows, 4)
	assert.Equal(t, worn{slot: "wielded", item: "a training dagger", power: 1, durability: 25, maxDurability: 25}, rows[0])
	assert.Equal(t, "about body", rows[2].slot, "a slot of two words")
	assert.Equal(t, worn{slot: "neck", item: "a temple token", power: 1}, rows[3], "no condition: never wears out")

	broken := readEquipment("Equipment (power 0)\n  wielded  an arming sword  power 4  broken\n")
	require.Len(t, broken, 1)
	assert.True(t, broken[0].broken)

	assert.Empty(t, readEquipment("Nothing equipped.\n"))
}

func TestNeedsRepair_atTheYellowMark(t *testing.T) {
	assert.False(t, worn{durability: 7, maxDurability: 20}.needsRepair(), "more than a third left")
	assert.True(t, worn{durability: 6, maxDurability: 20}.needsRepair(), "a third or less")
	assert.True(t, worn{broken: true, maxDurability: 20}.needsRepair())
	assert.False(t, worn{}.needsRepair(), "never wears out")
}

func TestReadPurse(t *testing.T) {
	assert.Equal(t, 37, readPurse("Inventory\n  a waterskin  power 1\nYou have 37 coins.\n"))
	assert.Equal(t, 1, readPurse("Inventory\n  a waterskin  power 1\nYou have 1 coin.\n"))
	assert.Zero(t, readPurse("Inventory\n  a waterskin  power 1\n"), "an empty purse says nothing")
	assert.Zero(t, readPurse(`Bob says, "You have 900 coins."`), "only its own purse line")
}

func TestNextPiece(t *testing.T) {
	rows := readEquipment(starterEquipment)
	require.NotNil(t, nextPiece(rows))
	assert.Equal(t, "arming sword", nextPiece(rows).name, "the starter dagger is the starter: replace it")

	rows[0] = worn{slot: "wielded", item: "an arming sword", power: 4}
	assert.Equal(t, "quilted jerkin", nextPiece(rows).name)

	full := []worn{}
	for _, p := range kit {
		full = append(full, worn{slot: p.slot, item: "a " + p.name, power: 4})
	}
	assert.Nil(t, nextPiece(full), "the whole kit on")
}

// A slot with something in it that the kit doesn't replace -- not its starter
// piece -- can't be cleared by the alias it knows, so buying for it would buy a
// piece it can't wear, again every visit.
func TestNextPiece_skipsASlotItCantClear(t *testing.T) {
	rows := []worn{
		{slot: "wielded", item: "an arming sword", power: 4},
		{slot: "body", item: "a quilted jerkin", power: 4},
		{slot: "head", item: "a bandit hood", power: 4},
	}
	assert.Equal(t, "studded boots", nextPiece(rows).name)
}

func TestReplies(t *testing.T) {
	m := buyRe.FindStringSubmatch("You buy an arming sword for 80 coins.\n")
	require.NotNil(t, m)
	assert.Equal(t, []string{"80", ""}, m[1:])
	m = buyRe.FindStringSubmatch("A quilted jerkin would cost 80 coins, and you have 12 coins.\n")
	require.NotNil(t, m)
	assert.Equal(t, []string{"", "80"}, m[1:])
	assert.Nil(t, buyRe.FindStringSubmatch(`Bob says, "You buy an arming sword for 1 coin."`))

	for _, said := range []string{
		"The smith works on a training dagger until it's as good as new, for 8 coins.\n",
		"A training dagger would cost 8 coins, and you have 0 coins.\n",
		"That doesn't need repairing.\n",
	} {
		assert.True(t, repairRe.MatchString(said), said)
	}
	assert.Equal(t, []string{"training", "wool", "travelers"}, spares())
}
