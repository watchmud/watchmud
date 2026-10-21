package world

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
	"uuid"
)

// bolt in testcontent: 20 mana, 15s cooldown, 6 + 3 per power. It is smite's
// blow under its own name -- what smite does, the smite suite pins; this one
// pins that bolt does the same, and says so in its own words.
type boltSuite struct {
	worldTestSuite
	clock  time.Time
	target *mobile.Instance
}

func TestBoltSuite(t *testing.T) {
	suite.Run(t, new(boltSuite))
}

func (s *boltSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
	var found bool
	s.target, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
}

// wield a wand granting bolt, at power 1, able to take 10 points of wear
func (s *boltSuite) wieldWand() *object.Instance {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), "wand", rules.SlotWield, rules.ObjectCategoryWeapon, rules.ArmorTypeNone)
	d.Damage = "1d4"
	d.Abilities = []string{"bolt"}
	d.MaxDurability = 10
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotWield, inst)
	return inst
}

func (s *boltSuite) bolt(target string) {
	s.T().Helper()
	s.r.Clear()
	cmd := command.Cast{Ability: "bolt", Target: target}
	s.w.handleCast(s.handlerParameter(cmd), cmd)
}

func (s *boltSuite) TestBolt() {
	s.wieldWand()

	s.bolt("target")

	s.Assert().Equal(16, s.target.CurHealth, "25 - (6 + 3)")
	s.Assert().Equal(event.Bolted{Actor: "testdood", Target: "Target Drone", Damage: 9},
		sent[event.Bolted](s.T(), s.r, 0))
	s.Assert().Equal(80, s.p.CurrentMana())
	s.Assert().Equal(s.clock.Add(15*time.Second), s.p.ReadyAt("bolt"))
	s.Assert().True(s.p.ReadyAt("smite").IsZero(), "bolt's cooldown is its own")
}

func (s *boltSuite) TestStartsAFight() {
	s.wieldWand()

	s.bolt("target")

	s.Require().True(s.w.fightLedger.IsFighting(s.p))
	s.Assert().Equal(s.target, s.w.fightLedger.GetFight(s.p).Fightee)
	s.Require().True(s.w.fightLedger.IsFighting(s.target))
	s.Assert().Equal(s.p, s.w.fightLedger.GetFight(s.target).Fightee)
}

func (s *boltSuite) TestKillingBolt() {
	s.wieldWand()
	s.target.CurHealth = 5

	s.bolt("target")

	_, alive := s.w.StartRoom.FindMobile("target")
	s.Assert().False(alive)
	s.Assert().False(s.w.fightLedger.InFight(s.p), "nobody left to fight")
	s.Assert().Equal(event.Bolted{Actor: "testdood", Target: "Target Drone", Damage: 9},
		sent[event.Bolted](s.T(), s.r, 0), "the bolt, then the death")
	s.Assert().Equal(event.Died{Target: "Target Drone"}, sent[event.Died](s.T(), s.r, 1))
}

func (s *boltSuite) TestWearsTheWand() {
	wand := s.wieldWand()

	s.bolt("target")

	s.Assert().Equal(9, wand.Durability)
}

func (s *boltSuite) TestNoFightRoom() {
	s.wieldWand()
	s.w.StartRoom.SetFlag(rules.RoomFlagNoFight)

	s.bolt("target")

	s.Assert().Equal(event.NoFightRoom, sent[event.Failed](s.T(), s.r, 0).Code)
	s.Assert().Equal(25, s.target.CurHealth)
	s.Assert().Equal(100, s.p.CurrentMana(), "a refusal costs no mana")
}
