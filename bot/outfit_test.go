package bot

import (
	"context"
	"strings"
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

func TestAffordable(t *testing.T) {
	a, _ := adventurer(t)
	rows := readEquipment(starterEquipment)
	assert.Equal(t, "arming sword", a.affordable(rows, 0).name, "a price it doesn't know is worth asking")
	a.prices["arming sword"] = 80
	assert.Nil(t, a.affordable(rows, 79), "known, and short")
	assert.NotNil(t, a.affordable(rows, 80))
}

// Two pieces in a row while the purse lasts: buy, take off what's in the
// slot, put on what it bought; then a price it learns by being refused.
func TestShop_buysWearsAndLearns(t *testing.T) {
	a, g := adventurer(t)
	a.prices["quilted jerkin"] = 80
	rows := readEquipment(starterEquipment)
	done := make(chan error, 1)
	go func() { done <- a.shop(context.Background(), rows, 170) }()

	assert.Equal(t, "buy arming", g.heard())
	g.say("You buy an arming sword for 80 coins.\r\n<100/100hp> ")
	assert.Equal(t, "remove training", g.heard())
	g.say("You stop using a training dagger.\r\n<100/100hp> ")
	assert.Equal(t, "wear arming", g.heard())
	g.say("You wear an arming sword.\r\n<100/100hp> ")

	assert.Equal(t, "buy quilted", g.heard(), "90 left covers the jerkin's 80")
	g.say("You buy a quilted jerkin for 80 coins.\r\n<100/100hp> ")
	assert.Equal(t, "remove wool", g.heard())
	g.say("You stop using a coarse wool tunic.\r\n<100/100hp> ")
	assert.Equal(t, "wear quilted", g.heard())
	g.say("You wear a quilted jerkin.\r\n<100/100hp> ")

	assert.Equal(t, "buy studded", g.heard(), "10 left, and the cap's price unknown: ask")
	g.say("A studded cap would cost 80 coins, and you have 10 coins.\r\n<100/100hp> ")
	require.NoError(t, <-done)

	assert.Equal(t, 80, a.prices["studded cap"], "learned by being refused")
	assert.Equal(t, 2, a.Stats().Bought)
}

// The cap's slot is empty: nothing to take off first.
func TestShop_aFreeSlot(t *testing.T) {
	a, g := adventurer(t)
	rows := []worn{
		{slot: "wielded", item: "an arming sword", power: 4},
		{slot: "body", item: "a quilted jerkin", power: 4},
	}
	a.prices["studded boots"] = 80
	done := make(chan error, 1)
	go func() { done <- a.shop(context.Background(), rows, 80) }()

	assert.Equal(t, "buy studded", g.heard())
	g.say("You buy a studded cap for 80 coins.\r\n<100/100hp> ")
	assert.Equal(t, "wear studded", g.heard())
	g.say("You wear a studded cap.\r\n<100/100hp> ")
	require.NoError(t, <-done, "nothing left for the boots' known 80")
}

// Only with coins to pay: a bot with worn gear and an empty purse would walk
// to the smith every visit to be told what it can't afford.
func TestWantsRepair(t *testing.T) {
	a, _ := adventurer(t)
	rows := readEquipment(starterEquipment) // the cloak is at 6/20
	assert.True(t, a.wantsRepair(rows, 10))
	assert.False(t, a.wantsRepair(rows, 0))
	assert.False(t, a.wantsRepair(readEquipment(starterEquipment)[:2], 10), "nothing at the mark")
}

// Refused once, it goes back only when the purse covers the cheapest piece
// the smith wouldn't mend -- not every visit to be told again.
func TestWantsRepair_notUntilItCanPay(t *testing.T) {
	a, _ := adventurer(t)
	rows := readEquipment(starterEquipment)
	a.repairShort = 7
	assert.False(t, a.wantsRepair(rows, 6), "still short")
	assert.True(t, a.wantsRepair(rows, 7))
}

func TestCheapestRefused(t *testing.T) {
	said := "The smith works on a training dagger until it's as good as new, for 2 coins.\n" +
		"A travelers cloak would cost 9 coins, and you have 3 coins.\n" +
		"A coarse wool tunic would cost 4 coins, and you have 3 coins.\n"
	assert.Equal(t, 4, cheapestRefused(said))
	assert.Zero(t, cheapestRefused("The smith works on a training dagger until it's as good as new, for 2 coins.\n"),
		"everything mended")
	assert.Zero(t, cheapestRefused(`Bob says, "A cloak would cost 1 coin, and you have 0 coins."`+"\n"),
		"only the smith's own lines")
}

// The donation trip drops the ground's loot and then the starter pieces --
// one still worn is an answer, not a lost bot.
func TestDonate_givesAwayTheStarterPieces(t *testing.T) {
	a, g := adventurer(t)
	a.ground = &grounds[0]
	done := make(chan error, 1)
	go func() { _, err := a.donate(context.Background()); done <- err }()

	assert.Equal(t, "recall", g.heard())
	g.say("Temple Square\r\n A square.\r\n[ Exits: e s ]\r\n<100/100hp> ")
	assert.Equal(t, "east", g.heard())
	g.say("Donation Room\r\n A room.\r\n[ Exits: w ]\r\n<100/100hp> ")
	assert.Equal(t, "inventory", g.heard())
	g.say("Inventory\r\n  a fat rat pelt  power 1\r\nYou have 9 coins.\r\n<100/100hp> ")
	assert.Equal(t, "equipment", g.heard(), "a farm bot has nothing to swap: it reads, then gives away what it always did")
	g.say(strings.ReplaceAll(starterEquipment, "\n", "\r\n") + "<100/100hp> ")
	for _, kw := range a.ground.loot {
		assert.Equal(t, "drop all."+kw, g.heard())
		g.say("You aren't carrying that.\r\n<100/100hp> ")
	}
	assert.Equal(t, "drop all.training", g.heard())
	g.say("You'll have to remove it first.\r\n<100/100hp> ")
	assert.Equal(t, "drop all.wool", g.heard())
	g.say("Dropped.\r\n<100/100hp> ")
	assert.Equal(t, "drop all.travelers", g.heard())
	g.say("You aren't carrying that.\r\n<100/100hp> ")
	assert.Equal(t, "west", g.heard())
	g.say("Temple Square\r\n A square.\r\n[ Exits: e s ]\r\n<100/100hp> ")
	require.NoError(t, <-done)
}

// The smith's refusal is remembered, so the next town visit with the same
// purse goes straight out.
func TestOutfit_learnsWhatRepairCosts(t *testing.T) {
	a, g := adventurer(t)
	a.here = "Market Square"
	a.prices["arming sword"] = 80 // nothing to buy with 3 coins
	rows := readEquipment(starterEquipment)
	go func() { _, _ = a.outfit(rows, 3)(context.Background()) }()

	assert.Equal(t, "west", g.heard())
	g.say("Smithy\r\n A forge.\r\n[ Exits: e ]\r\n<100/100hp> ")
	assert.Equal(t, "repair travelers", g.heard(), "the cloak, the one piece worn down")
	g.say("A travelers cloak would cost 7 coins, and you have 3 coins.\r\n<100/100hp> ")
	assert.Equal(t, "east", g.heard())
	g.say("Market Square\r\n A market.\r\n[ Exits: n e w ]\r\n<100/100hp> ")
	assert.Equal(t, "inventory", g.heard())
	g.say("Inventory\r\n  a waterskin  power 1\r\nYou have 3 coins.\r\n<100/100hp> ")
	assert.Equal(t, "equipment", g.heard(), "no store: the sword's 80 is out of reach")

	assert.Equal(t, 7, a.repairShort)
	assert.False(t, a.wantsRepair(rows, 3), "not back to the smith until it has 7")
}

// The word the smith is asked for a worn piece: its keyword if it's from the
// kit, the starter alias if it's what the kit replaces, nothing for the rest.
func TestRepairWord(t *testing.T) {
	rows := readEquipment(starterEquipment)
	assert.Equal(t, "training", rows[0].repairWord())
	assert.Equal(t, "wool", rows[1].repairWord())
	assert.Equal(t, "travelers", rows[2].repairWord())
	assert.Empty(t, rows[3].repairWord(), "the token: nothing in the kit for the neck")
	assert.Equal(t, "hobnailed", worn{slot: "feet", item: "a pair of studded boots"}.repairWord())
	assert.Empty(t, worn{slot: "head", item: "a bandit hood"}.repairWord(), "not the kit's")
}

// Each worn piece that has seen any wear, by name -- never "repair all",
// which would mend the starter pieces in the pack on their way to be donated.
// The cheapest refusal is what it remembers.
func TestOutfit_mendsWhatsWornByName(t *testing.T) {
	a, g := adventurer(t)
	a.here = "Market Square"
	a.prices["quilted jerkin"] = 80
	rows := []worn{
		{slot: "wielded", item: "an arming sword", power: 4, durability: 20, maxDurability: 25},
		{slot: "body", item: "a coarse wool tunic", power: 1, durability: 20, maxDurability: 20},
		{slot: "about body", item: "a travelers cloak", power: 1, durability: 5, maxDurability: 20},
		{slot: "head", item: "a studded cap", power: 4, broken: true, maxDurability: 20},
		{slot: "neck", item: "a temple token", power: 1},
	}
	go func() { _, _ = a.outfit(rows, 9)(context.Background()) }()

	assert.Equal(t, "west", g.heard())
	g.say("Smithy\r\n A forge.\r\n[ Exits: e ]\r\n<100/100hp> ")
	assert.Equal(t, "repair arming", g.heard(), "worn a little: mended while it's here")
	g.say("The smith works on an arming sword until it's as good as new, for 6 coins.\r\n<100/100hp> ")
	assert.Equal(t, "repair travelers", g.heard(), "the tunic is as new: not asked")
	g.say("A travelers cloak would cost 5 coins, and you have 3 coins.\r\n<100/100hp> ")
	assert.Equal(t, "repair studded", g.heard())
	g.say("A studded cap would cost 40 coins, and you have 3 coins.\r\n<100/100hp> ")
	assert.Equal(t, "east", g.heard(), "nothing asked for the token")
	g.say("Market Square\r\n A market.\r\n[ Exits: n e w ]\r\n<100/100hp> ")
	assert.Equal(t, "inventory", g.heard())
	g.say("Inventory\r\n  a waterskin  power 1\r\nYou have 3 coins.\r\n<100/100hp> ")
	assert.Equal(t, "equipment", g.heard())

	assert.Equal(t, 5, a.repairShort, "the cheaper of the two refusals")
}

// What it carries, as the inventory list shows it: a folded line is one entry
// with its count, and neither the heading nor the purse is an item.
func TestReadInventory(t *testing.T) {
	inv := readInventory("Inventory\n" +
		"  a leather bandit hood (x2)  power 4  20/20\n" +
		"  a knotted cudgel            power 5  25/25\n" +
		"  a studded cap               power 4  broken\n" +
		"  a long goose feather        power 1\n" +
		"You have 12 coins.\n")
	assert.Equal(t, []carried{
		{item: "a leather bandit hood", power: 4, count: 2},
		{item: "a knotted cudgel", power: 5, count: 1},
		{item: "a studded cap", power: 4, count: 1, broken: true},
		{item: "a long goose feather", power: 1, count: 1},
	}, inv)
	assert.Empty(t, readInventory("You aren't carrying anything.\nYou have 3 coins.\n"))
}

// A find being worn goes to the smith by its own keyword.
func TestRepairWord_finds(t *testing.T) {
	assert.Equal(t, "hood", worn{slot: "head", item: "a leather bandit hood"}.repairWord())
	assert.Equal(t, "cudgel", worn{slot: "wielded", item: "a knotted cudgel"}.repairWord())
	assert.Equal(t, "boar", worn{slot: "body", item: "a boar-hide jerkin"}.repairWord())
}

// What goes on: the best unbroken copy of a find, only above what the slot
// holds now, and only where it can take that off.
func TestUpgrades(t *testing.T) {
	kitOn := []worn{
		{slot: "wielded", item: "an arming sword", power: 4},
		{slot: "body", item: "a quilted jerkin", power: 4},
		{slot: "head", item: "a studded cap", power: 4},
	}
	pack := []carried{
		{item: "a leather bandit hood", power: 4, count: 2},
		{item: "a leather bandit hood", power: 5, count: 1},
		{item: "a knotted cudgel", power: 4, count: 1},
		{item: "a boar-hide jerkin", power: 6, count: 1, broken: true},
	}
	assert.Equal(t, []swap{{on: finds[0], power: 5, copies: 3, off: "studded"}}, upgrades(kitOn, pack),
		"the better hood only: an equal cudgel stays in the pack, a broken jerkin counts for nothing")

	oneHood := []carried{{item: "a leather bandit hood", power: 4, count: 1}}
	assert.Equal(t, []swap{{on: finds[0], power: 4, copies: 1, off: ""}}, upgrades(nil, oneHood),
		"an empty slot is power 0")
	brokenCap := []worn{{slot: "head", item: "a studded cap", power: 4, broken: true}}
	assert.Equal(t, []swap{{on: finds[0], power: 4, copies: 1, off: "studded"}}, upgrades(brokenCap, oneHood),
		"a broken row counts for nothing")
	strange := []worn{{slot: "head", item: "a jester's cap", power: 1}}
	assert.Empty(t, upgrades(strange, oneHood), "it can't take off what it can't name")
	assert.Empty(t, upgrades(kitOn, nil), "nothing found, nothing to wear")
}

// In the donation room, before it gives anything away: the better hood goes
// on, and the cap it replaces goes with the rest. Two hoods in the pack, the
// lesser first -- wear takes the first match -- so it looks before it wears,
// and gives the lesser away.
func TestDonate_wearsTheBestCopy(t *testing.T) {
	a, g := adventurer(t)
	a.ground = &grounds[2]
	done := make(chan error, 1)
	go func() { _, err := a.donate(context.Background()); done <- err }()

	assert.Equal(t, "recall", g.heard())
	g.say("Temple Square\r\n A square.\r\n[ Exits: e s ]\r\n<100/100hp> ")
	assert.Equal(t, "east", g.heard())
	g.say("Donation Room\r\n A room.\r\n[ Exits: w ]\r\n<100/100hp> ")
	assert.Equal(t, "inventory", g.heard())
	g.say("Inventory\r\n  a leather bandit hood  power 4  20/20\r\n  a leather bandit hood  power 5  20/20\r\nYou have 9 coins.\r\n<100/100hp> ")
	assert.Equal(t, "equipment", g.heard())
	g.say("Equipment (power 3)\r\n  head  a studded cap  power 4  20/20\r\n  neck  a temple token  power 1\r\n<100/100hp> ")
	assert.Equal(t, "remove studded", g.heard())
	g.say("You stop using a studded cap.\r\n<100/100hp> ")
	assert.Equal(t, "look hood", g.heard())
	g.say("A leather bandit hood\r\n  Leather armor, worn on the head. Power 4.\r\n<100/100hp> ")
	assert.Equal(t, "drop hood", g.heard(), "the lesser copy, first in the pack")
	g.say("Dropped.\r\n<100/100hp> ")
	assert.Equal(t, "look hood", g.heard())
	g.say("A leather bandit hood\r\n  Leather armor, worn on the head. Power 5.\r\n<100/100hp> ")
	assert.Equal(t, "wear hood", g.heard())
	g.say("You wear a leather bandit hood.\r\n<100/100hp> ")
	for _, kw := range []string{"hood", "cudgel", "training", "wool", "travelers", "studded"} {
		assert.Equal(t, "drop all."+kw, g.heard(), "the ground's loot, the starter spares, then what came off")
		g.say("You aren't carrying that.\r\n<100/100hp> ")
	}
	assert.Equal(t, "west", g.heard())
	g.say("Temple Square\r\n A square.\r\n[ Exits: e s ]\r\n<100/100hp> ")
	require.NoError(t, <-done)
	assert.Equal(t, 1, a.Stats().Upgrades)
}

// A miscounted pack -- the copy it meant to look at gone -- ends the swap
// with nothing worn rather than waiting on an answer that never comes.
func TestWearBest_nothingThere(t *testing.T) {
	a, g := adventurer(t)
	done := make(chan error, 1)
	go func() {
		_, err := a.wearBest(context.Background(), swap{on: finds[1], power: 5, copies: 1})
		done <- err
	}()
	assert.Equal(t, "look cudgel", g.heard())
	g.say("You don't see that here.\r\n<100/100hp> ")
	require.NoError(t, <-done)
	assert.Zero(t, a.Stats().Upgrades)
}
