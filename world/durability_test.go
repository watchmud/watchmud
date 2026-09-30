package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
)

// Gear wearing out in a real fight: a landed blow costs the defender a point
// of armor and the attacker a point of weapon, and what breaks stops counting
// without leaving the slot.
//
// Built on the world's own catalog rather than worldTestSuite, for the same
// reason the armor class suite is: player.NewTestPlayer's catalog has no
// armor table, so nothing it wears is worth anything.
type durabilitySuite struct {
	suite.Suite
	w     *World
	p     *player.Player
	r     *player.Recorder
	dice  *testdice.LoadedDice
	drone *mobile.Instance
}

func TestDurabilitySuite(t *testing.T) {
	suite.Run(t, new(durabilitySuite))
}

func (s *durabilitySuite) SetupTest() {
	w, err := NewTestWorld()
	s.Require().NoError(err)
	s.w = w

	cat := w.content.Catalog
	s.r = &player.Recorder{}
	s.p = player.New(uuid.New(), "victim", "", s.r, cat.DefaultLineage(), cat)
	s.w.PlacePlayer(s.p, s.w.StartRoom)

	drone, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	s.drone = drone

	s.dice = testdice.New()
	s.w.roller = s.dice
}

// equip a piece of gear with this much durability in it
func (s *durabilitySuite) wear(slot rules.EquipmentSlot, name string, t rules.ArmorType, maxDurability int) *object.Instance {
	armor := object.NewTestDefinition(s.T(),
		name,
		slot,
		rules.ObjectCategoryArmor,
		t,
	)
	armor.MaxDurability = maxDurability
	inst := object.NewInstance(uuid.New(), armor)
	s.p.Equipment().Equip(slot, inst)
	return inst
}

// oneWayFight leaves exactly one swing to take: the ledger writes both
// directions and DoViolence walks them in map order, which loaded dice cannot
// be aimed at.
func (s *durabilitySuite) oneWayFight(attacker, defender combat.Combatant) {
	s.T().Helper()
	s.w.fightLedger = combat.NewFightLedger()
	s.Require().NoError(s.w.fightLedger.Fight(attacker, defender))
	s.w.fightLedger.EndFight(defender)
}

// struck: the drone swings at the player with this d20. rolls are d20, then
// damage, then (on a hit, if the player has armor that can wear) the pick of
// which piece took it.
func (s *durabilitySuite) struck(rolls ...int) {
	s.dice.Load(rolls)
	s.oneWayFight(s.drone, s.p)
	s.r.Sent = nil
	s.w.DoViolence(5)
}

// swings: the player attacks the drone.
func (s *durabilitySuite) swings(rolls ...int) {
	s.dice.Load(rolls)
	s.oneWayFight(s.p, s.drone)
	s.r.Sent = nil
	s.w.DoViolence(5)
}

func (s *durabilitySuite) broke() (event.Broke, bool) {
	for _, msg := range s.r.Sent {
		if b, ok := msg.(event.Broke); ok {
			return b, true
		}
	}
	return event.Broke{}, false
}

func (s *durabilitySuite) TestALandedBlowWearsTheArmor() {
	plate := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 10)

	s.struck(20, 3, 0) // hits, 3 damage, wears the only piece there is

	s.Assert().Equal(9, plate.Durability)
}

// A miss costs nothing. The fights you lose ground in are the ones where
// something actually hit you.
func (s *durabilitySuite) TestAMissWearsNothing() {
	plate := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 10)

	s.struck(1, 3) // 1 does not reach AC 14

	s.Assert().Equal(10, plate.Durability)
}

// One piece per blow, not all of them: a full suit shouldn't wear out faster
// than a shirt.
func (s *durabilitySuite) TestOneBlowWearsOnePiece() {
	body := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 10)
	head := s.wear(rules.SlotHead, "iron helmet", rules.ArmorTypePlate, 10)

	// slot order is head-to-toe (rules.CompareSlots), so the helmet is the
	// first piece to choose between and the breastplate is the second.
	s.struck(20, 3, 1)

	s.Assert().Equal(9, body.Durability, "the blow landed on the body")
	s.Assert().Equal(10, head.Durability, "the helmet is untouched")
}

// The attacker's weapon pays for the blow it landed.
func (s *durabilitySuite) TestALandedBlowWearsTheWeapon() {
	knife := s.wear(rules.SlotWield, "knife", rules.ArmorTypeNone, 5)

	s.swings(20, 3) // the drone wears no armor, so nothing is picked

	s.Assert().Equal(4, knife.Durability)
}

// Gear with no durability at all is untouched, and doesn't even get asked
// about -- which is what keeps a server with no durability.json unchanged.
func (s *durabilitySuite) TestIndestructibleGearIsUntouched() {
	plate := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, rules.Indestructible)

	s.struck(20, 3) // no third roll: nothing to pick between

	s.Assert().Equal(0, plate.Durability)
	s.Assert().False(plate.Broken())
}

// The whole feature, end to end: the blow that finishes your breastplate
// takes your armor class and your role with it, and the room is told.
func (s *durabilitySuite) TestBreakingCostsArmorClassAndRole() {
	plate := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 1)
	s.Require().Equal(rules.BaseArmorClass+4, s.p.ArmorClass())
	s.Require().Equal("Tank", s.w.roleName(s.p.RoleWeights()))

	s.struck(20, 3, 0)

	s.Assert().True(plate.Broken())
	s.Assert().Equal(rules.BaseArmorClass, s.p.ArmorClass(), "broken armor protects nobody")
	s.Assert().Empty(s.w.roleName(s.p.RoleWeights()), "and argues for nothing")
	s.Assert().Equal(plate, s.p.Equipment().At(rules.SlotBody), "but is still worn")

	broke, told := s.broke()
	s.Require().True(told, "the room is told")
	s.Assert().Equal("victim", broke.Actor)
	s.Assert().Equal("plate mail", broke.Item, "the bare name, for the possessive")

	// and in that order: the blow, then what the blow finished. The other way
	// round reads like the armor gave out on its own.
	s.Require().Len(s.r.Sent, 2)
	s.Assert().IsType(event.Struck{}, s.r.Sent[0])
	s.Assert().IsType(event.Broke{}, s.r.Sent[1])
}

// Breaking is announced once. The piece keeps being worn and keeps being
// picked, and every blow after the first shouldn't re-announce it.
func (s *durabilitySuite) TestBreakingIsAnnouncedOnce() {
	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 1)

	s.struck(20, 3, 0)
	_, told := s.broke()
	s.Require().True(told)

	s.struck(20, 3) // broken armor is no longer in the pick, so no third roll
	_, toldAgain := s.broke()
	s.Assert().False(toldAgain)
}

// ---- dying --------------------------------------------------------------

// deathTable is the world's durability table with dying switched on, since
// testcontent deliberately has no durability.json at all.
func (s *durabilitySuite) deathTable(percent int) {
	s.w.content.Catalog.Durability = rules.DurabilityTable{
		Default:        40,
		OnDeathPercent: percent,
	}
}

// dies: the drone finishes the player off.
func (s *durabilitySuite) dies() {
	s.p.TakeMeleeDamage(s.p.CurrentHealth() - 1) // one point left
	s.dice.Load([]int{20, 5, 0})                 // hit, 5 damage, and the pick
	s.oneWayFight(s.drone, s.p)
	s.r.Sent = nil
	s.w.DoViolence(5)
	s.Require().True(s.died(), "the player died")
}

func (s *durabilitySuite) died() bool {
	for _, msg := range s.r.Sent {
		if d, ok := msg.(event.Died); ok && d.Target == s.p.Name() {
			return true
		}
	}
	return false
}

func (s *durabilitySuite) gearDamaged() (event.GearDamaged, bool) {
	for _, msg := range s.r.Sent {
		if g, ok := msg.(event.GearDamaged); ok {
			return g, true
		}
	}
	return event.GearDamaged{}, false
}

// Dying costs every piece you died in a share of what it started at -- not
// one piece, the way a blow does.
func (s *durabilitySuite) TestDyingCostsEveryPiece() {
	s.deathTable(10)
	body := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 80)
	head := s.wear(rules.SlotHead, "iron helmet", rules.ArmorTypePlate, 20)
	knife := s.wear(rules.SlotWield, "knife", rules.ArmorTypeNone, 40)

	s.dies()

	// The killing blow wears one piece of armor by a point on the way past --
	// the pick lands on the helmet, which is the first armor in slot order --
	// and then the death takes its share of all three.
	s.Assert().Equal(80-8, body.Durability)
	s.Assert().Equal(20-2-1, head.Durability, "and the blow that did it")
	s.Assert().Equal(40-4, knife.Durability, "the weapon dies with you too, though it threw nothing")
}

// The toll is told to the player, because the death where nothing breaks is
// the common one and would otherwise be invisible.
func (s *durabilitySuite) TestDyingTellsYouWhatItCost() {
	s.deathTable(10)
	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 80)
	s.wear(rules.SlotHead, "iron helmet", rules.ArmorTypePlate, 20)

	s.dies()

	damaged, told := s.gearDamaged()
	s.Require().True(told)
	s.Assert().Equal(2, damaged.Items)
}

// Dying in gear that was nearly finished finishes it, and that announces
// itself the same way it does in a fight.
func (s *durabilitySuite) TestDyingCanBreakGear() {
	s.deathTable(10)
	body := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 80)
	body.Damage(75) // 5 left, and dying costs 8
	s.Require().False(body.Broken())
	s.Require().Equal(rules.BaseArmorClass+4, s.p.ArmorClass())

	s.dies()

	s.Assert().True(body.Broken())
	s.Assert().Equal(rules.BaseArmorClass, s.p.ArmorClass())

	broke, told := s.broke()
	s.Require().True(told)
	s.Assert().Equal("plate mail", broke.Item)
}

// Nothing to lose, nothing said: indestructible gear pays nothing for dying,
// and the player isn't told about a toll that wasn't taken.
func (s *durabilitySuite) TestDyingInIndestructibleGearCostsNothing() {
	s.deathTable(10)
	body := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, rules.Indestructible)

	s.dies()

	s.Assert().Equal(0, body.Durability)
	s.Assert().False(body.Broken())
	_, told := s.gearDamaged()
	s.Assert().False(told)
}

// Content that doesn't switch dying on plays as it did before: the killing
// blow still wears a piece, the death itself costs nothing extra.
func (s *durabilitySuite) TestDyingIsFreeWhenContentSaysNothing() {
	s.deathTable(0)
	body := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 80)

	s.dies()

	s.Assert().Equal(79, body.Durability, "only the killing blow")
	_, told := s.gearDamaged()
	s.Assert().False(told)
}

// The order a player reads it in: the blow, the death, what the death cost,
// and then where they woke up.
func (s *durabilitySuite) TestTheTollFollowsTheDeath() {
	s.deathTable(10)
	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 80)

	s.dies()

	s.Require().Len(s.r.Sent, 4)
	s.Assert().IsType(event.Struck{}, s.r.Sent[0])
	s.Assert().IsType(event.Died{}, s.r.Sent[1])
	s.Assert().IsType(event.GearDamaged{}, s.r.Sent[2])
	s.Assert().IsType(event.RoomDescription{}, s.r.Sent[3])
}
