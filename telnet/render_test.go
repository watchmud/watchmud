package telnet

import (
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/command"
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
		want:  "Inventory\n  knife  power 0\n",
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
		want: "Players\n" +
			"  otherdood  Human  Temple Square, Wrathrock\n" +
			"  testdood   Human  Temple Square, Wrathrock\n" +
			"\n2 players online.\n",
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
		want: "Players\n" +
			"  otherdood  Human       Temple Square, Wrathrock\n" +
			"  testdood   Human Tank  Temple Square, Wrathrock\n" +
			"\n2 players online.\n",
	},
	{
		// bots are listed apart, after the people: which is which is the
		// server's, from the record, never something a client claims
		name: "who lists bots last",
		setup: func(_ *world.World, _ *player.Player, o *player.Player) {
			o.SetBot(true)
		},
		input: "who",
		want: "Players\n" +
			"  testdood   Human  Temple Square, Wrathrock\n" +
			"\nBots\n" +
			"  otherdood  Human  Temple Square, Wrathrock\n" +
			"\n1 player and 1 bot online.\n",
	},
	{
		// recall moves with direction.None, which must not render as "none!"
		name:      "recall leaves in no direction",
		setup:     func(w *world.World, p *player.Player, _ *player.Player) { wearTestToken(w, p) },
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
		name:      "smite a mob",
		setup:     func(_ *world.World, p *player.Player, _ *player.Player) { wieldTestMace(p) },
		input:     "cast smite little",
		want:      "You smite Little Drone! (11)\n",
		wantOther: "testdood smites Little Drone!\n",
	},
	{
		name:  "smite with no fight and no target",
		setup: func(_ *world.World, p *player.Player, _ *player.Player) { wieldTestMace(p) },
		input: "cast smite",
		want:  "At what? You aren't fighting anything.\n",
	},
	{
		name:  "recall without a token",
		input: "recall",
		want:  "Nothing you're wearing lets you recall -- a temple token does; the General Store sells them.\n",
	},
	{
		name:      "provoke a mob",
		setup:     func(_ *world.World, p *player.Player, _ *player.Player) { wearTestPlate(p) },
		input:     "cast provoke little",
		want:      "You provoke Little Drone, and it turns on you!\n",
		wantOther: "testdood provokes Little Drone!\n",
	},
	{
		name:      "stun a mob",
		setup:     func(_ *world.World, p *player.Player, _ *player.Player) { wieldTestStunMace(p) },
		input:     "cast stun little",
		want:      "You stun Little Drone!\n",
		wantOther: "testdood stuns Little Drone!\n",
	},
	{
		name:      "assess a mob",
		setup:     func(_ *world.World, p *player.Player, _ *player.Player) { wearTestAssessHood(p) },
		input:     "cast assess little",
		want:      "You study Little Drone.\n  Health 25/25  AC 10  Power 1  Hits for 1d2\n",
		wantOther: "testdood studies Little Drone.\n",
	},
	{
		name:      "ward yourself",
		setup:     func(_ *world.World, p *player.Player, _ *player.Player) { wearTestWardRing(p) },
		input:     "cast ward",
		want:      "A ward settles over you. (12)\n",
		wantOther: "testdood casts ward.\n",
	},
	{
		name:      "ward another",
		setup:     func(_ *world.World, p *player.Player, _ *player.Player) { wearTestWardRing(p) },
		input:     "cast ward otherdood",
		want:      "You ward otherdood. (12)\n",
		wantOther: "testdood wards you. (12)\n",
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
		want:  "Abilities\n  heal  20 mana  10s cooldown  a censer (power 1)  ready\n",
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

// wieldTestMace wields something granting smite at power 1
func wieldTestMace(p *player.Player) {
	d := object.NewDefinition("mace", "mace", "wrathrock", rules.ObjectCategoryWeapon,
		nil, "a mace", "A mace is here.", rules.SlotWield, rules.ArmorTypeNone, nil)
	d.Damage = "1d6"
	d.Abilities = []string{"smite"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	_ = p.Inventory().Add(inst)
	p.Equipment().Equip(rules.SlotWield, inst)
}

// wearTestPlate wears something granting provoke
func wearTestPlate(p *player.Player) {
	d := object.NewDefinition("breastplate", "breastplate", "wrathrock", rules.ObjectCategoryArmor,
		nil, "a breastplate", "A breastplate is here.", rules.SlotBody, rules.ArmorTypePlate, nil)
	d.Abilities = []string{"provoke"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	_ = p.Inventory().Add(inst)
	p.Equipment().Equip(rules.SlotBody, inst)
}

// wieldTestStunMace wields something granting stun
func wieldTestStunMace(p *player.Player) {
	d := object.NewDefinition("mace", "mace", "wrathrock", rules.ObjectCategoryWeapon,
		nil, "a mace", "A mace is here.", rules.SlotWield, rules.ArmorTypeNone, nil)
	d.Damage = "1d8"
	d.Abilities = []string{"stun"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	_ = p.Inventory().Add(inst)
	p.Equipment().Equip(rules.SlotWield, inst)
}

// wearTestAssessHood puts a hood granting assess on p's head
func wearTestAssessHood(p *player.Player) {
	d := object.NewDefinition("hood", "hood", "wrathrock", rules.ObjectCategoryArmor,
		nil, "a hood", "A hood is here.", rules.SlotHead, rules.ArmorTypeLeather, nil)
	d.Abilities = []string{"assess"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	_ = p.Inventory().Add(inst)
	p.Equipment().Equip(rules.SlotHead, inst)
}

// wearTestWardRing puts a ring granting ward on p's finger
func wearTestWardRing(p *player.Player) {
	d := object.NewDefinition("ring", "ring", "wrathrock", rules.ObjectCategoryTreasure,
		nil, "a ring", "A ring is here.", rules.SlotFingers, rules.ArmorTypeNone, nil)
	d.Abilities = []string{"ward"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	_ = p.Inventory().Add(inst)
	p.Equipment().Equip(rules.SlotFingers, inst)
}

// wearTestToken puts testcontent's temple token, which grants recall, on p's neck
func wearTestToken(w *world.World, p *player.Player) {
	d, _ := w.ObjectDefinition("wrathrock", "temple_token")
	inst := object.NewInstance(uuid.New(), d)
	_ = p.Inventory().Add(inst)
	p.Equipment().Equip(d.EquipmentSlot, inst)
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

// No ability block: what a character can do is their gear, and the six
// scores were a number nothing read.
const statBlockNoGear = "testdood, the Human\n" +
	"  Health       100/100\n" +
	"  Mana         100/100\n" +
	"  Power        0\n" +
	"  Armor class  10\n" +
	"  Coins        0\n" +
	"  Where        Temple Square, Wrathrock\n"

func TestRenderPurse(t *testing.T) {
	assert.Equal(t, "You aren't carrying anything.\n", plain(render(event.Inventory{}, "testdood")), "an empty purse says nothing")
	assert.Equal(t, "You aren't carrying anything.\nYou have 1 coin.\n", plain(render(event.Inventory{Coins: 1}, "testdood")))
	assert.Equal(t, "You aren't carrying anything.\nYou have 30 coins.\n", plain(render(event.Inventory{Coins: 30}, "testdood")))
}

func TestRenderShopList(t *testing.T) {
	got := render(event.ShopList{Items: []event.ShopEntry{
		{Item: "a short sword", Power: 2, Price: 40},
		{Item: "a waterskin", Power: 1, Price: 10},
	}}, "testdood")
	assert.Equal(t, "For sale\n"+
		"  a short sword  power 2  40 coins\n"+
		"  a waterskin    power 1  10 coins\n", plain(got))
	assert.Equal(t, "You buy a short sword for 40 coins.\n", render(event.Bought{Item: "a short sword", Cost: 40}, "testdood"))
	assert.Equal(t, "You sell a scrap of rat pelt for 4 coins.\n", render(event.Sold{Item: "a scrap of rat pelt", Coins: 4}, "testdood"))
	assert.Equal(t, "The shopkeeper would give you 1 coin for a feather.\n", render(event.Valued{Item: "a feather", Coins: 1}, "testdood"))
}

// a provoke on what's already fighting you is wasted, and says so
func TestRender_provokeAlready(t *testing.T) {
	m := event.Provoked{Actor: "testdood", Target: "Little Drone", Already: true}
	assert.Equal(t, "Little Drone is already fighting you.\n", plain(render(m, "testdood")))
	assert.Equal(t, "testdood provokes Little Drone!\n", plain(render(m, "otherdood")))
}

// a stunned mob's lost round
func TestRender_staggered(t *testing.T) {
	assert.Equal(t, "Little Drone staggers, stunned.\n", plain(render(event.Staggered{Name: "Little Drone"}, "testdood")))
}

func TestRender_summoned(t *testing.T) {
	assert.Equal(t, "Barrow-King calls up 2 barrow skeletons!\n",
		plain(render(event.Summoned{Summoner: "Barrow-King", Name: "barrow skeleton", Count: 2}, "testdood")))
	assert.Equal(t, "Warlock calls up one imp!\n",
		plain(render(event.Summoned{Summoner: "Warlock", Name: "imp", Count: 1}, "testdood")))
}

func TestRender_crumbled(t *testing.T) {
	assert.Equal(t, "barrow skeleton crumbles to dust.\n",
		plain(render(event.Crumbled{Name: "barrow skeleton"}, "testdood")))
}

// recall's refusals in its own words, not cast's
func TestFailure_recall(t *testing.T) {
	assert.Equal(t, "Nothing you're wearing lets you recall -- a temple token does; the General Store sells them.\n",
		plain(render(event.Failed{Verb: "recall", Code: event.NotGranted}, "testdood")))
	assert.Equal(t, "You can't recall again yet.\n",
		plain(render(event.Failed{Verb: "recall", Code: event.NotReady}, "testdood")))
}

// the temple token handed to a character from before it
func TestRender_received(t *testing.T) {
	assert.Equal(t, "You find a temple token around your neck.\n",
		plain(render(event.Received{Item: "a temple token", Worn: true}, "testdood")))
	assert.Equal(t, "You find a temple token in your pack. Wear it to recall.\n",
		plain(render(event.Received{Item: "a temple token", Worn: false}, "testdood")))
}

// wear names what went on: "wear leather" could be a cap or boots
func TestRender_worn(t *testing.T) {
	assert.Equal(t, "You wear a pair of leather boots.\n",
		plain(render(event.Worn{Item: "a pair of leather boots"}, "testdood")))
}

func TestRender_gold(t *testing.T) {
	assert.Equal(t, "250 coins appear in your purse. You have 750.\n",
		plain(render(event.GoldGiven{Amount: 250, Coins: 750}, "testdood")))
	assert.Equal(t, "Gold how much? A number from 1 to a million.\n",
		plain(render(event.Failed{Verb: "gold", Code: event.BadRequest}, "testdood")))
}

func TestRender_equipped(t *testing.T) {
	assert.Equal(t, "You wield a knife.\n", plain(render(event.Equipped{Item: "a knife"}, "testdood")))
}

// a blow on a warded player, as each end and a bystander read it
func TestRender_struckThroughAWard(t *testing.T) {
	part := event.Struck{Attacker: "Little Drone", Target: "testdood", Hit: true, Damage: 3, Absorbed: 4}
	assert.Equal(t, "Little Drone hits you for 3 damage; your ward takes 4.\n", plain(render(part, "testdood")))
	assert.Equal(t, "Little Drone hits testdood.\n", plain(render(part, "otherdood")))

	all := event.Struck{Attacker: "Little Drone", Target: "testdood", Hit: true, Absorbed: 5}
	assert.Equal(t, "Little Drone hits you, but your ward takes it. (5)\n", plain(render(all, "testdood")))
	assert.Equal(t, "Little Drone hits testdood.\n", plain(render(all, "otherdood")))
}

func TestRender_wardBroken(t *testing.T) {
	m := event.WardBroken{Name: "testdood"}
	assert.Equal(t, "Your ward shatters.\n", plain(render(m, "testdood")))
	assert.Equal(t, "testdood's ward shatters.\n", plain(render(m, "otherdood")))
}

// mid-fight, an assess says who the mob is on and how long its stun lasts
func TestRender_assessedMidFight(t *testing.T) {
	m := event.Assessed{Actor: "testdood", Target: "Barrow-King", Health: 132, MaxHealth: 180,
		ArmorClass: 16, Power: 15, Damage: "2d8", Fighting: "otherdood", Stunned: 1}
	assert.Equal(t, "You study Barrow-King.\n"+
		"  Health 132/180  AC 16  Power 15  Hits for 2d8\n"+
		"  Fighting otherdood\n"+
		"  Stunned for 1 more round\n", plain(render(m, "testdood")))
	assert.Equal(t, "testdood studies Barrow-King.\n", plain(render(m, "otherdood")))

	m.Fighting, m.Stunned = "testdood", 2
	assert.Equal(t, "You study Barrow-King.\n"+
		"  Health 132/180  AC 16  Power 15  Hits for 2d8\n"+
		"  Fighting you\n"+
		"  Stunned for 2 more rounds\n", plain(render(m, "testdood")))
}

const statBlockTank = "testdood, the Human Tank\n" +
	"  Health       100/100\n" +
	"  Mana         100/100\n" +
	"  Power        0\n" +
	"  Armor class  10\n" +
	"  Coins        0\n" +
	"  Where        Temple Square, Wrathrock\n"

const statBlockPowered = "testdood, the Human Tank\n" +
	"  Health       100/100\n" +
	"  Mana         100/100\n" +
	"  Power        7\n" +
	"  Armor class  10\n" +
	"  Coins        0\n" +
	"  Where        Temple Square, Wrathrock\n"

// The same thing in the same shape is one line with a count; a different
// power or condition is a different line.
func TestRenderInventory_groups(t *testing.T) {
	feather := event.InventoryItem{ShortDescription: "a long goose feather", Power: 1}
	worn := event.InventoryItem{ShortDescription: "a knife", Power: 2, Durability: 20, MaxDurability: 25}
	got := plain(render(event.Inventory{Items: []event.InventoryItem{
		feather, worn, feather, feather,
		{ShortDescription: "a long goose feather", Power: 3},
	}, Coins: 12}, "testdood"))

	assert.Equal(t, "Inventory\n"+
		"  a long goose feather (x3)  power 1\n"+
		"  a knife                    power 2  20/25\n"+
		"  a long goose feather       power 3\n"+
		"You have 12 coins.\n", got)
}

// A bag says how much it holds, so two empty ones fold and a full one doesn't.
func TestRenderInventory_bags(t *testing.T) {
	empty := event.InventoryItem{ShortDescription: "a leather satchel", Power: 1, Bag: true}
	full := event.InventoryItem{ShortDescription: "a leather satchel", Power: 1, Bag: true, Holding: 3}
	got := plain(render(event.Inventory{Items: []event.InventoryItem{empty, full, empty}}, "testdood"))

	assert.Equal(t, "Inventory\n"+
		"  a leather satchel (x2) (empty)  power 1\n"+
		"  a leather satchel (3 inside)    power 1\n", got)
}

// A door: the one who did it, the room that watched, and the far side, which
// hears it without seeing who.
func TestRender_doorChanged(t *testing.T) {
	opened := event.DoorChanged{Actor: "testdood", Door: "iron grate", Direction: rules.DirectionWest, Change: event.DoorOpened}
	assert.Equal(t, "You open the iron grate.\n", plain(render(opened, "testdood")))
	assert.Equal(t, "testdood opens the iron grate.\n", plain(render(opened, "otherdood")))

	far := event.DoorChanged{Door: "iron grate", Direction: rules.DirectionEast, Change: event.DoorOpened}
	assert.Equal(t, "The iron grate to the east opens.\n", plain(render(far, "otherdood")))
	far.Change = event.DoorUnlocked
	assert.Equal(t, "You hear a click from the iron grate to the east.\n", plain(render(far, "otherdood")))
	far.Direction, far.Change = rules.DirectionUp, event.DoorClosed
	assert.Equal(t, "The iron grate above closes.\n", plain(render(far, "otherdood")))

	assert.Equal(t, "Exits:\neast, west (closed)\n", plain(render(event.Exits{Exits: []event.Exit{
		{Direction: rules.DirectionEast}, {Direction: rules.DirectionWest, Closed: true},
	}}, "testdood")))
}

func TestRender_containerChanged(t *testing.T) {
	m := event.ContainerChanged{Actor: "testdood", Container: "strongbox", Change: event.DoorUnlocked}
	assert.Equal(t, "You unlock the strongbox.\n", plain(render(m, "testdood")))
	assert.Equal(t, "testdood unlocks the strongbox.\n", plain(render(m, "otherdood")))
}

// "put x in y", "put x into y"; a missing container is the handler's to answer
func TestParse_put(t *testing.T) {
	for line, want := range map[string]command.Put{
		"put knife in chest":        {Target: "knife", Into: "chest"},
		"put all.pelt into the bin": {Target: "all.pelt", Into: "the bin"},
		"put 20 coins in strongbox": {Target: "20 coins", Into: "strongbox"},
		"put knife":                 {Target: "knife"},
	} {
		cmd, err := parseCommand(strings.Fields(line))
		require.NoError(t, err, line)
		assert.Equal(t, want, cmd, line)
	}
	m := event.Put{Actor: "testdood", Item: "a knife", Into: "a strongbox"}
	assert.Equal(t, "You put a knife in a strongbox.\n", plain(render(m, "testdood")))
	assert.Equal(t, "testdood puts a knife in a strongbox.\n", plain(render(m, "otherdood")))
}

// "give x to y"; the giver, the one given to, and the room
func TestParse_give(t *testing.T) {
	for line, want := range map[string]command.Give{
		"give knife to bob":    {Target: "knife", To: "bob"},
		"give 20 coins to Bob": {Target: "20 coins", To: "Bob"},
		"give all.pelt to bob": {Target: "all.pelt", To: "bob"},
		"give knife":           {Target: "knife"},
	} {
		cmd, err := parseCommand(strings.Fields(line))
		require.NoError(t, err, line)
		assert.Equal(t, want, cmd, line)
	}
	m := event.Gave{Actor: "testdood", Recipient: "bob", Item: "a knife"}
	assert.Equal(t, "You give a knife to bob.\n", plain(render(m, "testdood")))
	assert.Equal(t, "testdood gives you a knife.\n", plain(render(m, "bob")))
	assert.Equal(t, "testdood gives a knife to bob.\n", plain(render(m, "otherdood")))
	assert.Equal(t, "Give it to whom?\n", failureText("give", "NO_RECIPIENT"))
}

// look <thing>: an object, a mob, a player
func TestRender_lookedAt(t *testing.T) {
	hood := event.LookedAtObject{Item: "a bandit hood", Slot: rules.SlotHead, ArmorType: rules.ArmorTypeLeather,
		Worn: true, Power: 3, Durability: 12, MaxDurability: 20, Armor: 1, Abilities: []string{"assess"}}
	assert.Equal(t, "A bandit hood\n"+
		"  Leather armor, worn on the head. Power 3.\n"+
		"  Condition 12/20.\n"+
		"  Adds 1 to armor class.\n"+
		"  Lets you cast assess.\n"+
		"  You have it on.\n", plain(render(hood, "testdood")))

	knife := event.LookedAtObject{Item: "a knife", Slot: rules.SlotWield, Power: 1, Damage: "1d4", Broken: true, MaxDurability: 20}
	assert.Equal(t, "A knife\n  A weapon. Power 1.\n  Condition broken.\n  Hits for 1d4.\n", plain(render(knife, "testdood")))

	bag := event.LookedAtObject{Item: "a leather satchel", Power: 1, Container: true, Holding: 3, Capacity: 10}
	assert.Equal(t, "A leather satchel\n  Power 1.\n  Holding 3 of 10.\n", plain(render(bag, "testdood")))
	box := event.LookedAtObject{Item: "a strongbox", Container: true, Closed: true, Locked: true}
	assert.Contains(t, plain(render(box, "testdood")), "It's locked.")

	rat := event.LookedAtMob{Name: "giant rat", Health: 40, Fighting: "bob"}
	assert.Equal(t, "The giant rat is badly hurt, fighting bob.\n", plain(render(rat, "testdood")))
	assert.Equal(t, "The giant rat is in perfect health.\n", plain(render(event.LookedAtMob{Name: "giant rat", Health: 100}, "testdood")))

	bob := event.LookedAtPlayer{Name: "Bob", Lineage: "Wood Elf", Role: "Tank", Health: 80, Wearing: []string{"a knife", "a leather cap"}}
	assert.Equal(t, "Bob is a Wood Elf, slightly hurt.\n  Geared as a Tank.\n  Wearing a knife, a leather cap.\n",
		plain(render(bob, "testdood")))
	assert.Contains(t, plain(render(event.LookedAtPlayer{Name: "Ann", Lineage: "Orc", Health: 100}, "Ann")), "You are an Orc, in perfect health.")

	for _, line := range []string{"look knife", "examine knife", "exa knife"} {
		cmd, err := parseCommand(strings.Fields(line))
		require.NoError(t, err)
		assert.Equal(t, command.Look{Target: "knife"}, cmd, line)
	}
}

// groups: follow, the walk after, the list, and gtell
func TestRender_groups(t *testing.T) {
	f := event.Following{Follower: "ann", Leader: "bob"}
	assert.Equal(t, "You now follow bob.\n", plain(render(f, "ann")))
	assert.Equal(t, "ann now follows you.\n", plain(render(f, "bob")))
	f.Stopped = true
	assert.Equal(t, "You stop following bob.\n", plain(render(f, "ann")))
	assert.Equal(t, "ann stops following you.\n", plain(render(f, "bob")))

	assert.Equal(t, "You follow bob north.\n", plain(render(event.Followed{Leader: "bob", Direction: rules.DirectionNorth}, "ann")))
	assert.Equal(t, "bob leaves north, but you can't follow in the middle of a fight.\n",
		plain(render(event.Followed{Leader: "bob", Direction: rules.DirectionNorth, Fighting: true}, "ann")))

	list := event.GroupList{Members: []event.GroupMember{
		{Name: "bob", Leader: true, Health: 97, MaxHealth: 100, Mana: 80, MaxMana: 100, Room: "The Mill Yard"},
		{Name: "ann", Health: 5, MaxHealth: 100, Mana: 100, MaxMana: 100, Room: "Temple Square"},
	}}
	assert.Equal(t, "Group\n"+
		"  bob (leader)  97/100hp   80/100m  The Mill Yard\n"+
		"  ann            5/100hp  100/100m  Temple Square\n", plain(render(list, "ann")))

	told := event.GroupTold{Speaker: "ann", Value: "ready?"}
	assert.Equal(t, "You tell the group, 'ready?'\n", plain(render(told, "ann")))
	assert.Equal(t, "ann tells the group, 'ready?'\n", plain(render(told, "bob")))

	for line, want := range map[string]command.Command{
		"follow bob":   command.Follow{Target: "bob"},
		"follow":       command.Follow{},
		"ungroup ann":  command.Ungroup{Target: "ann"},
		"group":        command.Group{},
		"gt on my way": command.GroupTell{Value: "on my way"},
		"gtell ready?": command.GroupTell{Value: "ready?"},
	} {
		cmd, err := parseCommand(strings.Fields(line))
		require.NoError(t, err, line)
		assert.Equal(t, want, cmd, line)
	}
}

func TestRender_swept(t *testing.T) {
	assert.Equal(t, "The janitor sweeps up a pelt and tips it into a barrow.\n",
		plain(render(event.Swept{Sweeper: "janitor", Item: "a pelt"}, "testdood")))
}

func TestRender_assist(t *testing.T) {
	m := event.Assisted{Actor: "ann", Member: "bob", Target: "wolf"}
	assert.Equal(t, "You leap to bob's aid against the wolf!\n", plain(render(m, "ann")))
	assert.Equal(t, "ann leaps to bob's aid against the wolf!\n", plain(render(m, "bob")))
	assert.Equal(t, "You'll join your group's fights.\n", plain(render(event.AssistSet{On: true}, "ann")))
	assert.Equal(t, "You'll stay out of your group's fights unless you join in.\n", plain(render(event.AssistSet{}, "ann")))
	cmd, err := parseCommand(strings.Fields("assist OFF"))
	require.NoError(t, err)
	assert.Equal(t, command.Assist{Setting: "off"}, cmd)
}

func TestRender_ooc(t *testing.T) {
	assert.Equal(t, "[ooc] ann: anyone for the mill?\n", plain(render(event.OOCSaid{Speaker: "ann", Value: "anyone for the mill?"}, "bob")))
	assert.Equal(t, "You've left the ooc channel. 'ooc on' to come back.\n", plain(render(event.OOCSet{}, "bob")))
	for _, line := range []string{"ooc hi there", "newbie hi there", "nb hi there"} {
		cmd, err := parseCommand(strings.Fields(line))
		require.NoError(t, err)
		assert.Equal(t, command.OOC{Value: "hi there"}, cmd, line)
	}
}

func TestRender_snatched(t *testing.T) {
	assert.Equal(t, "The crow snatches up a goose feather.\n",
		plain(render(event.Snatched{Mob: "crow", Item: "a goose feather"}, "testdood")))
}
