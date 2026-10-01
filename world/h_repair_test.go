package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type handleRepairSuite struct {
	worldTestSuite
}

func TestHandleRepairSuite(t *testing.T) {
	suite.Run(t, new(handleRepairSuite))
}

func (s *handleRepairSuite) toSmithy() {
	s.T().Helper()
	smithy, found := s.w.findRoomById("wrathrock", "smithy")
	s.Require().True(found)
	s.w.movePlayerMagically(s.p, smithy)
	s.r.Sent = nil     // the description of where they landed
	s.p.AddCoins(1000) // what it costs is TestRepair_costs's business
}

func (s *handleRepairSuite) repair(target string) {
	s.T().Helper()
	cmd := command.Repair{Target: target}
	s.w.handleRepair(s.handlerParameter(cmd), cmd)
}

// carry something with this much durability left out of max
func (s *handleRepairSuite) carry(name string, left, max int) *object.Instance {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), name, rules.SlotBody, rules.ObjectCategoryArmor, rules.ArmorTypeLeather)
	d.MaxDurability = max
	inst := object.NewInstance(uuid.New(), d)
	inst.Durability = left
	s.Require().NoError(s.p.Inventory().Add(inst))
	return inst
}

func (s *handleRepairSuite) TestRepair() {
	s.toSmithy()
	vest := s.carry("vest", 3, 40)

	s.repair("vest")

	s.Assert().Equal(40, vest.Durability)
	s.Assert().Equal("vest", sent[event.Repaired](s.T(), s.r, 0).Item)
}

// Broken is not destroyed, and this is what that was for.
func (s *handleRepairSuite) TestRepair_broken() {
	s.toSmithy()
	vest := s.carry("vest", 0, 40)
	s.Require().True(vest.Broken())

	s.repair("vest")

	s.Assert().False(vest.Broken())
	s.Assert().Equal(40, vest.Durability)
}

// What's worn is in the inventory too, so it's found the same way.
func (s *handleRepairSuite) TestRepair_whatYouHaveOn() {
	s.toSmithy()
	vest := s.carry("vest", 5, 40)
	s.p.Equipment().Equip(rules.SlotBody, vest)

	s.repair("vest")

	s.Assert().Equal(40, vest.Durability)
}

// all mends what needs it and passes over the rest without a word.
func (s *handleRepairSuite) TestRepair_all() {
	s.toSmithy()
	vest := s.carry("vest", 5, 40)
	s.carry("cloak", 20, 20)
	s.carry("ring", 0, rules.Indestructible)
	boots := s.carry("boots", 1, 20)

	s.repair("all")

	s.Assert().Equal(40, vest.Durability)
	s.Assert().Equal(20, boots.Durability)
	s.Require().Len(s.r.Sent, 2)
	s.Assert().Equal("vest", sent[event.Repaired](s.T(), s.r, 0).Item)
	s.Assert().Equal("boots", sent[event.Repaired](s.T(), s.r, 1).Item)
}

func (s *handleRepairSuite) TestRepair_notDamaged() {
	s.toSmithy()
	s.carry("cloak", 20, 20)

	s.repair("cloak")
	s.Assert().Equal(event.NotDamaged, sent[event.Failed](s.T(), s.r, 0).Code)

	s.repair("all")
	s.Assert().Equal(event.NotDamaged, sent[event.Failed](s.T(), s.r, 1).Code)
}

func (s *handleRepairSuite) TestRepair_onlyAtASmithy() {
	vest := s.carry("vest", 3, 40)

	s.repair("vest")

	s.Assert().Equal(3, vest.Durability)
	s.Assert().Equal(event.NoSmith, sent[event.Failed](s.T(), s.r, 0).Code)
}

func (s *handleRepairSuite) TestRepair_notCarried() {
	s.toSmithy()

	s.repair("vest")
	s.Assert().Equal(event.TargetNotFound, sent[event.Failed](s.T(), s.r, 0).Code)

	s.repair("")
	s.Assert().Equal(event.NoTarget, sent[event.Failed](s.T(), s.r, 1).Code)
}

// testcontent's economy: armor is worth 20 a power, a full repair is half
// that, and less worn is proportionally less.
func (s *handleRepairSuite) TestRepair_costs() {
	s.toSmithy()
	s.p.Spend(s.p.Coins())
	s.p.AddCoins(30)
	vest := s.carry("vest", 0, 40) // broken, power 0 priced as 1: 20 x 50%
	vest.Power = 2                 // 40 x 50%

	s.repair("vest")

	s.Assert().Equal(event.Repaired{Item: "vest", Cost: 20}, sent[event.Repaired](s.T(), s.r, 0))
	s.Assert().Equal(10, s.p.Coins())
}

// Short of coins, the smith says what it would cost and mends nothing.
func (s *handleRepairSuite) TestRepair_cantAfford() {
	s.toSmithy()
	s.p.Spend(s.p.Coins())
	s.p.AddCoins(3)
	vest := s.carry("vest", 0, 40)

	s.repair("vest")

	s.Assert().True(vest.Broken())
	s.Assert().Equal(3, s.p.Coins())
	s.Assert().Equal(event.TooExpensive{Item: "vest", Cost: 10, Coins: 3}, sent[event.TooExpensive](s.T(), s.r, 0))
}

// repair all pays as it goes: what it can afford, in order, and a word about
// the rest.
func (s *handleRepairSuite) TestRepair_allWithTooLittle() {
	s.toSmithy()
	s.p.Spend(s.p.Coins())
	s.p.AddCoins(12)
	vest := s.carry("vest", 0, 40)   // 10
	boots := s.carry("boots", 0, 20) // 10: can't, by then
	helm := s.carry("helm", 10, 20)  // 5: can't either, with 2 left

	s.repair("all")

	s.Assert().False(vest.Broken())
	s.Assert().True(boots.Broken())
	s.Assert().Equal(10, helm.Durability)
	s.Assert().Equal(2, s.p.Coins())
	s.Require().Len(s.r.Sent, 3)
	s.Assert().Equal("vest", sent[event.Repaired](s.T(), s.r, 0).Item)
	s.Assert().Equal("boots", sent[event.TooExpensive](s.T(), s.r, 1).Item)
	s.Assert().Equal("helm", sent[event.TooExpensive](s.T(), s.r, 2).Item)
}
