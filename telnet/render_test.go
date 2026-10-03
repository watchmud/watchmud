package telnet

import (
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/world"
)

type commandCase struct {
	name      string
	setup     func(*world.World, *player.Player, *player.Player)
	input     string
	want      string
	wantOther string
	// wantParseError is set for input the parser rejects before the world
	// sees it; the connection shows the player this text directly.
	wantParseError string
}

var commandCases = []commandCase{
	{
		name:  "get nonexistent",
		input: "get nonexistent",
		want:  "You don't see that here.\n",
	},
	{
		name:  "list away from a shop",
		input: "list",
		want:  "There's no one here to trade with. Try the General Store in Wrathrock.\n",
	},
	{
		name:  "sell away from a shop",
		input: "sell knife",
		want:  "There's no one here to trade with. Try the General Store in Wrathrock.\n",
	},
	{
		name:  "repair away from a smithy",
		input: "repair knife",
		want:  "There's no one here who can repair that. Try the smithy in Wrathrock.\n",
	},
	{
		name:  "color off",
		input: "color off",
		want:  "Color is off.\n",
	},
	{
		name:  "color with nonsense",
		input: "color purple",
		want:  "Color on, color off, or just color to switch it.\n",
	},
	{
		name:  "look shows the room",
		input: "look",
		want:  startRoomBlock,
	},
	{
		name:  "move with no exit",
		input: "north",
		want:  "You can't go that way.\n",
	},
	{
		name:  "get something that isn't here",
		input: "get sword",
		want:  "You don't see that here.\n",
	},
	{
		name:      "get from the room",
		input:     "get knife",
		want:      "Taken.\n",
		wantOther: "testdood gets knife.\n",
	},
	{
		// the target grammar reaches get, not just drop: "all", "all.knife"
		// and "2.knife" are world.parseTarget's job either way
		name:      "get all",
		input:     "get all",
		want:      "Taken.\nTaken.\n",
		wantOther: "testdood gets iron helmet.\ntestdood gets knife.\n",
	},
	{
		name:      "get all of one name",
		input:     "get all.knife",
		want:      "Taken.\n",
		wantOther: "testdood gets knife.\n",
	},
	{
		name:  "get the second one when there is only one",
		input: "get 2.knife",
		want:  "You don't see that here.\n",
	},
	{
		name: "drop all",
		setup: func(_ *world.World, p *player.Player, o *player.Player) {
			p.Inventory().Add(testKnife())
			p.Inventory().Add(testHelmet())
		},
		input:     "drop all",
		want:      "Dropped.\nDropped.\n",
		wantOther: "testdood drops knife.\ntestdood drops iron helmet.\n",
	},
	{
		// the parse error is ours; the player gets one failure message
		name:  "a target the grammar can't read",
		input: "get x.knife",
		want:  "You'll have to phrase that differently.\n",
	},
	{
		// the verb override in resultcode.go: TARGET_NOT_FOUND means
		// something different to drop than it does to get
		name:  "drop something you aren't carrying",
		input: "drop sword",
		want:  "You aren't carrying that.\n",
	},
	{
		name:      "drop what you have",
		setup:     func(_ *world.World, p *player.Player, o *player.Player) { p.Inventory().Add(testKnife()) },
		input:     "drop knife",
		want:      "Dropped.\n",
		wantOther: "testdood drops knife.\n",
	},
	{
		name:  "empty inventory",
		input: "inventory",
		want:  "You aren't carrying anything.\n",
	},
	{
		name:  "inventory with one item",
		setup: func(_ *world.World, p *player.Player, o *player.Player) { p.Inventory().Add(testKnife()) },
		input: "inv",
		want:  "You are carrying:\n\tknife\n",
	},
	{
		name:      "say reaches the room",
		input:     "say hello there",
		want:      "You say, \"hello there\".\n",
		wantOther: "testdood says, \"hello there\".\n",
	},
	{
		name:  "tell to someone who isn't playing",
		input: "tell nobody hi",
		want:  "No one by that name is playing.\n",
	},
	{
		name:      "tell reaches the target",
		input:     "tell otherdood hi",
		want:      "Ok.\n",
		wantOther: "testdood tells you, \"hi\".\n",
	},
	{
		name:      "tellall reaches everyone else",
		input:     "tellall listen up",
		want:      "Ok.\n",
		wantOther: "testdood shouts, \"listen up\".\n",
	},
	{
		name:  "tellall with nothing to say",
		input: "tellall",
		want:  "Say what?\n",
	},
	{
		name:  "exits",
		input: "exits",
		want:  "Exits:\neast, south\n",
	},
	{
		name:  "who lists everyone",
		input: "who",
		want:  "-- Who Is Here --\notherdood the Human - Temple Square - Wrathrock\ntestdood the Human - Temple Square - Wrathrock\n",
	},
	{
		name:  "nothing equipped",
		input: "equipment",
		want:  "Nothing equipped.\n",
	},
	{
		name:  "wear something you aren't carrying",
		input: "wear helmet",
		want:  "You aren't carrying that.\n",
	},
	{
		name:  "wield with no target",
		input: "wield",
		want:  "Wield what?\n",
	},
	{
		name:  "kill something that isn't here",
		input: "kill dragon",
		want:  "You don't see that here.\n",
	},
	{
		name:  "a verb nobody knows",
		input: "florb the thing",
		want:  "",
		// parseCommand rejects it before the world ever sees it; the
		// connection prints the parser's error itself.
		wantParseError: "Unknown request: florb",
	},
	{
		name:  "role with nothing equipped",
		input: "role",
		want:  roleBlockNoGear,
	},
	{
		// the phase in one test case: gear alone decides the role
		name: "wearing armor makes you a tank",
		setup: func(_ *world.World, p *player.Player, o *player.Player) {
			p.Equipment().Equip(rules.SlotHead, testHelmet())
		},
		input: "role",
		want:  roleBlockTank,
	},
	{
		name: "wielding a knife makes you a striker",
		setup: func(_ *world.World, p *player.Player, o *player.Player) {
			p.Equipment().Equip(rules.SlotWield, testKnife())
		},
		input: "roles",
		want:  roleBlockStriker,
	},
	{
		name: "remove something you aren't using",
		setup: func(_ *world.World, p *player.Player, o *player.Player) {
			p.Inventory().Add(testHelmet()) // carrying it is not using it
		},
		input: "remove helmet",
		want:  "You aren't using that.\n",
	},
	{
		name:  "remove with no target",
		input: "remove",
		want:  "Remove what?\n",
	},
	{
		name: "taking the gear off takes the role with it",
		setup: func(_ *world.World, p *player.Player, o *player.Player) {
			p.Equipment().Equip(rules.SlotHead, testHelmet())
		},
		input: "remove helmet",
		want:  "You stop using iron helmet.\n",
	},
	{
		name:  "stat shows lineage and role, not class",
		input: "stat",
		want:  statBlockNoGear,
	},
	{
		name: "stat reflects what is equipped right now",
		setup: func(_ *world.World, p *player.Player, o *player.Player) {
			p.Equipment().Equip(rules.SlotHead, testHelmet())
		},
		input: "stat",
		want:  statBlockTank,
	},
	{
		name: "stat shows the power of what is worn",
		setup: func(_ *world.World, p *player.Player, o *player.Player) {
			helmet := testHelmet()
			helmet.Power = 7
			p.Equipment().Equip(rules.SlotHead, helmet)
		},
		input: "stat",
		want:  statBlockPowered,
	},
	{
		// testcontent: the target drone is power 3, the little one power 1,
		// and a player wearing nothing is power 0
		name:  "consider something above you",
		input: "consider target",
		want:  "Target Drone would be a real challenge. (power 3; you are 0)\n",
	},
	{
		name:  "consider something about level",
		input: "con little",
		want:  "Little Drone looks like a fair fight. (power 1; you are 0)\n",
	},
	{
		name:  "consider something that isn't here",
		input: "consider dragon",
		want:  "You don't see that here.\n",
	},
	{
		name:  "consider with no target",
		input: "consider",
		want:  "Consider what?\n",
	},
	{
		// a role shows up in who beside the lineage, where a class used to
		name: "who shows the role",
		setup: func(_ *world.World, p *player.Player, o *player.Player) {
			p.Equipment().Equip(rules.SlotHead, testHelmet())
		},
		input: "who",
		want:  "-- Who Is Here --\notherdood the Human - Temple Square - Wrathrock\ntestdood the Human Tank - Temple Square - Wrathrock\n",
	},
	{
		// a bot is labelled where players look for who is around: the label
		// is the server's, from the record, never something a client claims
		name: "who marks bots",
		setup: func(_ *world.World, _ *player.Player, o *player.Player) {
			o.SetBot(true)
		},
		input: "who",
		want:  "-- Who Is Here --\notherdood [bot] the Human - Temple Square - Wrathrock\ntestdood the Human - Temple Square - Wrathrock\n",
	},
	{
		// recall moves with direction.None, which must not render as "none!"
		name:      "recall leaves in no direction",
		input:     "recall",
		want:      startRoomBlock,
		wantOther: "testdood leaves.\ntestdood enters.\n",
	},
	{
		name: "cast heal on another",
		setup: func(w *world.World, p *player.Player, o *player.Player) {
			holdTestCenser(p)
			o.TakeMeleeDamage(50)
		},
		input:     "cast heal otherdood",
		want:      "You heal otherdood. (+12)\n",
		wantOther: "testdood heals you. (+12)\n",
	},
	{
		name: "c is cast, and heal on yourself",
		setup: func(w *world.World, p *player.Player, _ *player.Player) {
			holdTestCenser(p)
			p.TakeMeleeDamage(50)
		},
		input:     "c heal",
		want:      "You heal yourself. (+12)\n",
		wantOther: "testdood casts heal.\n",
	},
	{
		name:      "healing the unhurt is wasted",
		setup:     func(_ *world.World, p *player.Player, _ *player.Player) { holdTestCenser(p) },
		input:     "cast heal otherdood",
		want:      "You heal otherdood, but otherdood wasn't hurt.\n",
		wantOther: "testdood heals you, but you weren't hurt.\n",
	},
	{
		name:  "cast without the gear",
		input: "cast heal",
		want:  "Nothing you're wearing lets you cast that.\n",
	},
	{
		name:  "cast nothing",
		input: "cast",
		want:  "Cast what?\n",
	},
	{
		name:  "cast at someone not here",
		setup: func(_ *world.World, p *player.Player, _ *player.Player) { holdTestCenser(p) },
		input: "cast heal nobody",
		want:  "There's no one here by that name.\n",
	},
	{
		// a builder command refused to a player reads like a verb that
		// doesn't exist -- the same words parse.go uses for one
		name:  "load is unknown to a player",
		input: "load mob rabbit",
		want:  "Unknown request: load\n",
	},
	{
		name: "load works for a wizard",
		setup: func(_ *world.World, p *player.Player, _ *player.Player) {
			p.SetWizard(true)
		},
		input: "load mob rabbit",
		want:  "Loaded.\n",
	},
	{
		name: "slay",
		setup: func(_ *world.World, p *player.Player, _ *player.Player) {
			p.SetWizard(true)
		},
		input:     "slay little",
		want:      "You slay Little Drone.\nLittle Drone is dead!\n",
		wantOther: "testdood slays Little Drone.\nLittle Drone is dead!\n",
	},
	{
		name:  "slay is unknown to a player",
		input: "slay little",
		want:  "Unknown request: slay\n",
	},
	{
		name:  "nohassle is unknown to a player",
		input: "nohassle",
		want:  "Unknown request: nohassle\n",
	},
	{
		// the test world never runs Arrive, so a bare nohassle flips it on
		name: "nohassle for a wizard",
		setup: func(_ *world.World, p *player.Player, _ *player.Player) {
			p.SetWizard(true)
		},
		input: "nohassle",
		want:  "Aggressive mobs will leave you alone.\n",
	},
	{
		name: "nohassle off",
		setup: func(_ *world.World, p *player.Player, _ *player.Player) {
			p.SetWizard(true)
		},
		input: "nohassle off",
		want:  "Aggressive mobs can see you again.\n",
	},
	{
		name:  "abilities with nothing",
		input: "abilities",
		want:  "Nothing you're wearing grants any abilities.\n",
	},
	{
		name:  "abilities lists what the gear grants",
		setup: func(_ *world.World, p *player.Player, _ *player.Player) { holdTestCenser(p) },
		input: "abilities",
		want:  "heal         20 mana  10s cooldown  a censer (power 1)  ready\n",
	},
}

func TestCommandRendering(t *testing.T) {
	for _, tc := range slices.Concat(commandCases, lootCases, scriptCases) {
		t.Run(tc.name, func(t *testing.T) {
			w, err := world.NewTestWorld()
			require.NoError(t, err)

			rec := &player.Recorder{}
			p := player.NewTestPlayer(uuid.New(), "testdood", rec)
			w.PlacePlayer(p, w.StartRoom)
			c := gameserver.NewTestConn(p)

			// second player
			otherRec := &player.Recorder{}
			o := player.NewTestPlayer(uuid.New(), "otherdood", otherRec)
			w.PlacePlayer(o, w.StartRoom)
			_ = gameserver.NewTestConn(o)

			if tc.setup != nil {
				tc.setup(w, p, o)
			}

			// the same path the connection takes: parse, then dispatch.
			cmd, err := parseCommand(strings.Fields(tc.input))
			if tc.wantParseError != "" {
				require.EqualError(t, err, tc.wantParseError)
				return
			}
			require.NoError(t, err)
			require.NoError(t, w.HandleIncomingMessage(gameserver.NewHandlerParameter(c, cmd)))

			var got strings.Builder
			for _, m := range rec.Sent {
				got.WriteString(plain(render(m, p.Name())))
			}
			assert.Equal(t, tc.want, got.String())

			var otherGot strings.Builder
			for _, m := range otherRec.Sent {
				otherGot.WriteString(plain(render(m, o.Name())))
			}
			assert.Equal(t, tc.wantOther, otherGot.String())
		})
	}
}

// what NewTestWorld's start room looks like. The floor lists newest first
// (object.NewFloor), so a freshly reset zone shows its objects in the reverse
// of the order instructions.json creates them in; mobs and players are in the
// order they arrived.
const startRoomBlock = `Temple Square
 The main square of the town. People come and go. East is a donation room, south is the marketplace.
[ Exits: East, South ]
A plain iron helmet lies here.
A knife is on the ground.
Target Drone buzzes around.
Little Drone buzzes around.
otherdood is here.
`

func testKnife() *object.Instance {
	d := object.NewDefinition(
		"knife",
		"knife",
		"start",
		rules.ObjectCategoryWeapon,
		[]string{},
		"knife",
		"A knife is on the ground.",
		rules.SlotWield,
		rules.ArmorTypeCloth,
		rules.EmptyObjectBehaviors,
	)
	d.RoleWeights = map[string]int{"striker": 2}
	return object.NewInstance(uuid.New(), d)
}

// holdTestCenser equips something granting heal at power 1
func holdTestCenser(p *player.Player) {
	d := object.NewDefinition("censer", "censer", "wrathrock", rules.ObjectCategoryOther,
		nil, "a censer", "A censer is here.", rules.SlotHold, rules.ArmorTypeNone, nil)
	d.Abilities = []string{"heal"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	_ = p.Inventory().Add(inst)
	p.Equipment().Equip(rules.SlotHold, inst)
}

func testHelmet() *object.Instance {
	d := object.NewDefinition(
		"helmet",
		"helmet",
		"start",
		rules.ObjectCategoryArmor,
		[]string{"helm"},
		"iron helmet",
		"an iron helmet is on the ground",
		rules.SlotHead,
		rules.ArmorTypeCloth,
		rules.EmptyObjectBehaviors,
	)
	d.RoleWeights = map[string]int{"tank": 2}
	return object.NewInstance(uuid.New(), d)
}

// The role listing always shows every role, including the ones with nothing
// behind them: "you are a Tank" with no standings is a verdict the player
// can't argue with or work out how to change.
const roleBlockNoGear = `You aren't wearing anything that argues for a role.
  Tank     0
  Healer   0
  Striker  0
Change what you're wearing to change your role.
`

const roleBlockTank = `You are fighting as a Tank.
 Armored to the teeth and standing between the fight and everyone else.
  Tank     2  (iron helmet 2)
  Healer   0
  Striker  0
Change what you're wearing to change your role.
`

const roleBlockStriker = `You are fighting as a Striker.
 Carrying something sharp and no reason to be careful with it.
  Tank     0
  Healer   0
  Striker  2  (knife 2)
Change what you're wearing to change your role.
`

// Tabs, so these are quoted rather than raw. There is no ability block: what
// a character can do is their gear, and the six scores were a number nothing
// read.
const statBlockNoGear = "Status:\n" +
	"Player:\ttestdood\n" +
	"Lineage:\tHuman\tRole: none\n" +
	"Power:\t0\n" +
	"Health:\t100 of 100\n" +
	"Coins:\t0\n" +
	"Location:\t(wrathrock - temple_square)\n\n"

const statBlockTank = "Status:\n" +
	"Player:\ttestdood\n" +
	"Lineage:\tHuman\tRole: Tank\n" +
	"Power:\t0\n" +
	"Health:\t100 of 100\n" +
	"Coins:\t0\n" +
	"Location:\t(wrathrock - temple_square)\n\n"

const statBlockPowered = "Status:\n" +
	"Player:\ttestdood\n" +
	"Lineage:\tHuman\tRole: Tank\n" +
	"Power:\t7\n" +
	"Health:\t100 of 100\n" +
	"Coins:\t0\n" +
	"Location:\t(wrathrock - temple_square)\n\n"

func TestRenderPurse(t *testing.T) {
	assert.Equal(t, "You aren't carrying anything.\n", render(event.Inventory{}, "testdood"), "an empty purse says nothing")
	assert.Equal(t, "You aren't carrying anything.\nYou have 1 coin.\n", render(event.Inventory{Coins: 1}, "testdood"))
	assert.Equal(t, "You aren't carrying anything.\nYou have 30 coins.\n", render(event.Inventory{Coins: 30}, "testdood"))
}

func TestRenderShopList(t *testing.T) {
	got := render(event.ShopList{Items: []event.ShopEntry{
		{Item: "a short sword", Power: 2, Price: 40},
		{Item: "a waterskin", Power: 1, Price: 10},
	}}, "testdood")
	assert.Equal(t, "For sale here:\n"+
		"  a short sword  [power 2]  40 coins\n"+
		"  a waterskin    [power 1]  10 coins\n", got)
	assert.Equal(t, "You buy a short sword for 40 coins.\n", render(event.Bought{Item: "a short sword", Cost: 40}, "testdood"))
	assert.Equal(t, "You sell a scrap of rat pelt for 4 coins.\n", render(event.Sold{Item: "a scrap of rat pelt", Coins: 4}, "testdood"))
	assert.Equal(t, "The shopkeeper would give you 1 coin for a feather.\n", render(event.Valued{Item: "a feather", Coins: 1}, "testdood"))
}
