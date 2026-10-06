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
	"github.com/watchmud/watchmud/rules"
)

// assess in testcontent: 10 mana, 10s cooldown, target mob. The start room
// has a Target Drone (25 health, ac 10, power 3) and a Little Drone.
type assessSuite struct {
	worldTestSuite
	clock  time.Time
	target *mobile.Instance
	little *mobile.Instance
}

func TestAssessSuite(t *testing.T) {
	suite.Run(t, new(assessSuite))
}

func (s *assessSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
	var found bool
	s.target, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.little, found = s.w.StartRoom.FindMobile("little")
	s.Require().True(found)

	d := object.NewTestDefinition(s.T(), "hood", rules.SlotHead, rules.ObjectCategoryArmor, rules.ArmorTypeLeather)
	d.Abilities = []string{"assess"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotHead, inst)
}

func (s *assessSuite) assess(target string) {
	s.T().Helper()
	s.r.Clear()
	cmd := command.Cast{Ability: "assess", Target: target}
	s.w.handleCast(s.handlerParameter(cmd), cmd)
}

func (s *assessSuite) TestAssess() {
	s.target.CurHealth = 17

	s.assess("target")

	s.Assert().Equal(event.Assessed{
		Actor:      "testdood",
		Target:     "Target Drone",
		Health:     17,
		MaxHealth:  25,
		ArmorClass: 10,
		Power:      3,
		Damage:     s.target.WeaponDamageRoll(),
	}, sent[event.Assessed](s.T(), s.r, 0))
	s.Assert().Equal(90, s.p.CurrentMana())
	s.Assert().Equal(s.clock.Add(10*time.Second), s.p.ReadyAt("assess"))
}

// looking isn't fighting: no fight starts
func (s *assessSuite) TestStartsNoFight() {
	s.assess("target")

	s.Assert().False(s.w.fightLedger.InFight(s.p))
	s.Assert().False(s.w.fightLedger.InFight(s.target))
}

// mid-fight, bare: whoever you're fighting, who it's on, and its stun
func (s *assessSuite) TestMidFight() {
	s.Require().NoError(s.w.startFight(s.p, s.target))
	s.w.fightLedger.Stun(s.target, 2)

	s.assess("")

	a := sent[event.Assessed](s.T(), s.r, 0)
	s.Assert().Equal("Target Drone", a.Target)
	s.Assert().Equal("testdood", a.Fighting)
	s.Assert().Equal(2, a.Stunned)
}

func (s *assessSuite) TestBareWithNoFight() {
	s.assess("")

	s.Assert().Equal(event.NoFoe, sent[event.Failed](s.T(), s.r, 0).Code)
	s.Assert().Equal(100, s.p.CurrentMana(), "nothing spent")
}

func (s *assessSuite) TestNotHere() {
	s.assess("dragon")

	s.Assert().Equal(event.TargetNotFound, sent[event.Failed](s.T(), s.r, 0).Code)
}

// what kill and smite refuse, assess doesn't: a no-fight room, a mob that
// can't be fought
func (s *assessSuite) TestNotRefusedForAFight() {
	s.w.StartRoom.SetFlag(rules.RoomFlagNoFight)
	s.little.Definition.SetFlag(rules.MobileFlagPlayerCantFight)

	s.assess("little")

	s.Assert().Equal("Little Drone", sent[event.Assessed](s.T(), s.r, 0).Target)
}
