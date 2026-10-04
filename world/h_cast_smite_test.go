package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// smite in testcontent: 25 mana, 20s cooldown, 8 + 3 per power. The start
// room has a Target Drone and a Little Drone, 25 health each.
type smiteSuite struct {
	worldTestSuite
	clock  time.Time
	target *mobile.Instance
	little *mobile.Instance
}

func TestSmiteSuite(t *testing.T) {
	suite.Run(t, new(smiteSuite))
}

func (s *smiteSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
	var found bool
	s.target, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.little, found = s.w.StartRoom.FindMobile("little")
	s.Require().True(found)
}

// wield a mace granting smite, at this power, able to take 10 points of wear
func (s *smiteSuite) wieldMace(power int) *object.Instance {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), "mace", rules.SlotWield, rules.ObjectCategoryWeapon, rules.ArmorTypeNone)
	d.Damage = "1d6"
	d.Abilities = []string{"smite"}
	d.MaxDurability = 10
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = power
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotWield, inst)
	return inst
}

func (s *smiteSuite) smite(target string) {
	s.T().Helper()
	s.r.Clear()
	cmd := command.Cast{Ability: "smite", Target: target}
	s.w.handleCast(s.handlerParameter(cmd), cmd)
}

func (s *smiteSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

// nothing was spent: full mana, no cooldown
func (s *smiteSuite) assertNothingSpent() {
	s.T().Helper()
	s.Assert().Equal(100, s.p.CurrentMana(), "a refusal costs no mana")
	s.Assert().True(s.p.ReadyAt("smite").IsZero(), "and starts no cooldown")
}

func (s *smiteSuite) TestSmite() {
	s.wieldMace(1)

	s.smite("target")

	s.Assert().Equal(14, s.target.CurHealth, "25 - (8 + 3)")
	s.Assert().Equal(event.Smote{Actor: "testdood", Target: "Target Drone", Damage: 11},
		sent[event.Smote](s.T(), s.r, 0))
	s.Assert().Equal(75, s.p.CurrentMana())
	s.Assert().Equal(s.clock.Add(20*time.Second), s.p.ReadyAt("smite"))
}

// it opens a fight, like kill: you're fighting it and it's fighting you
func (s *smiteSuite) TestStartsAFight() {
	s.wieldMace(1)

	s.smite("target")

	s.Require().True(s.w.fightLedger.IsFighting(s.p))
	s.Assert().Equal(s.target, s.w.fightLedger.GetFight(s.p).Fightee)
	s.Require().True(s.w.fightLedger.IsFighting(s.target))
	s.Assert().Equal(s.p, s.w.fightLedger.GetFight(s.target).Fightee)
}

// mid-fight, a bare smite hits whoever you're fighting
func (s *smiteSuite) TestBareSmiteHitsYourTarget() {
	s.wieldMace(1)
	s.Require().NoError(s.w.startFight(s.p, s.little))

	s.smite("")

	s.Assert().Equal(14, s.little.CurHealth)
	s.Assert().Equal(25, s.target.CurHealth)
}

// smiting someone else mid-fight hits them and draws them in, but your own
// swings stay on the one you were fighting
func (s *smiteSuite) TestSmiteAnotherMidFightKeepsYourTarget() {
	s.wieldMace(1)
	s.Require().NoError(s.w.startFight(s.p, s.little))

	s.smite("target")

	s.Assert().Equal(14, s.target.CurHealth)
	s.Assert().Equal(s.little, s.w.fightLedger.GetFight(s.p).Fightee, "still swinging at the little one")
	s.Require().True(s.w.fightLedger.IsFighting(s.target))
	s.Assert().Equal(s.p, s.w.fightLedger.GetFight(s.target).Fightee, "and the target drone has turned on you")
}

// a smite that kills is a real death: corpse, and no fight left behind
func (s *smiteSuite) TestKillingSmite() {
	s.wieldMace(1)
	s.target.CurHealth = 5

	s.smite("target")

	_, alive := s.w.StartRoom.FindMobile("target")
	s.Assert().False(alive)
	s.Assert().Len(s.w.StartRoom.Inventory.FindAll("corpse"), 1)
	s.Assert().False(s.w.fightLedger.InFight(s.p), "nobody left to fight")
	s.Assert().Equal(event.Smote{Actor: "testdood", Target: "Target Drone", Damage: 11},
		sent[event.Smote](s.T(), s.r, 0), "the blow, then the death")
	s.Assert().Equal(event.Died{Target: "Target Drone"}, sent[event.Died](s.T(), s.r, 1))
}

// a smite is a landed blow, and wears the weapon like one
func (s *smiteSuite) TestWearsTheWeapon() {
	mace := s.wieldMace(1)

	s.smite("target")

	s.Assert().Equal(9, mace.Durability)
}

// with no fight to aim at, a bare smite has to be told what to hit -- NO_FOE,
// since the cast verb's NO_TARGET would read "Cast what?" to someone who
// just named the spell
func (s *smiteSuite) TestBareSmiteWithNoFight() {
	s.wieldMace(1)

	s.smite("")

	s.Assert().Equal(event.NoFoe, s.failed())
	s.assertNothingSpent()
}

func (s *smiteSuite) TestUnknownMob() {
	s.wieldMace(1)

	s.smite("dragon")

	s.Assert().Equal(event.TargetNotFound, s.failed())
	s.assertNothingSpent()
}

// foe means a mob: a player standing here isn't one
func (s *smiteSuite) TestNotAPlayer() {
	s.wieldMace(1)
	bob := player.NewTestPlayer(uuid.New(), "bob", nil)
	s.w.PlacePlayer(bob, s.w.StartRoom)

	s.smite("bob")

	s.Assert().Equal(event.TargetNotFound, s.failed())
	s.Assert().Equal(100, bob.CurrentHealth())
	s.assertNothingSpent()
}

func (s *smiteSuite) TestNoFightRoom() {
	s.wieldMace(1)
	s.w.StartRoom.SetFlag(rules.RoomFlagNoFight)

	s.smite("target")

	s.Assert().Equal(event.NoFightRoom, s.failed())
	s.Assert().Equal(25, s.target.CurHealth)
	s.assertNothingSpent()
}

func (s *smiteSuite) TestCantFightMob() {
	s.wieldMace(1)
	s.target.Definition.SetFlag(rules.MobileFlagPlayerCantFight)

	s.smite("target")

	s.Assert().Equal(event.NoFight, s.failed())
	s.Assert().Equal(25, s.target.CurHealth)
	s.assertNothingSpent()
}

// a stronger weapon hits harder: the power is the granting item's
func (s *smiteSuite) TestPowerScales() {
	s.wieldMace(4)

	s.smite("target")

	s.Assert().Equal(5, s.target.CurHealth, "25 - (8 + 12)")
}
