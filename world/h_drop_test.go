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

type HandleDropSuite struct {
	worldTestSuite
}

func TestHandleDropSuite(t *testing.T) {
	suite.Run(t, new(HandleDropSuite))
}

func (s *HandleDropSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
}

// get picks something up so there is something to drop.
func (s *HandleDropSuite) get(target string) {
	s.T().Helper()
	cmd := command.Get{Target: target}
	s.w.handleGet(s.handlerParameter(cmd), cmd)
}

func (s *HandleDropSuite) drop(target string) {
	s.T().Helper()
	cmd := command.Drop{Target: target}
	s.w.handleDrop(s.handlerParameter(cmd), cmd)
}

func (s *HandleDropSuite) TestSuccess() {
	s.get("knife")
	s.drop("knife")

	dropped := sent[event.Dropped](s.T(), s.r, 1)
	s.Assert().Equal("testdood", dropped.Actor)
	s.Assert().Equal("knife", dropped.Item)

	// player now has zero items, room has its starting two
	s.Assert().Equal(0, s.p.Inventory().Len())
	s.Assert().Equal(2, s.w.StartRoom.Inventory.Len())
}

func (s *HandleDropSuite) TestAlias() {
	s.get("helmet")
	s.drop("helmet")

	s.Assert().Equal(2, len(s.r.Sent))
	sent[event.Dropped](s.T(), s.r, 1)

	// player now has zero items, room has its starting two
	s.Assert().Equal(0, s.p.Inventory().Len())
	s.Assert().Equal(2, s.w.StartRoom.Inventory.Len())
}

func (s *HandleDropSuite) TestNoTarget() {
	s.drop("")

	s.Assert().Equal(1, len(s.r.Sent))
	failed := sent[event.Failed](s.T(), s.r, 0)
	s.Assert().Equal("drop", failed.Verb)
	s.Assert().Equal(event.NoTarget, failed.Code)
}

func (s *HandleDropSuite) TestNotFound() {
	// drop (but you don't have one)
	s.drop("knife")

	s.Assert().Equal(1, len(s.r.Sent))
	failed := sent[event.Failed](s.T(), s.r, 0)
	s.Assert().Equal("drop", failed.Verb)
	s.Assert().Equal(event.TargetNotFound, failed.Code)
	s.Assert().Equal(0, s.p.Inventory().Len())
}

func (s *HandleDropSuite) TestInUse() {
	s.get("knife")

	// now wield the knife
	equip := command.Equip{Target: "knife", Slot: rules.SlotWield}
	s.w.handleEquip(s.handlerParameter(equip), equip)

	s.drop("knife")

	failed := sent[event.Failed](s.T(), s.r, 2)
	s.Assert().Equal(event.TargetInUse, failed.Code)
	s.Assert().Equal(1, s.p.Inventory().Len())
}

func (s *HandleDropSuite) TestInUseMultipleItems() {
	// drop knife
	// - there are two knives on you,
	//   one you are holding, another in your inventory list.
	// - drop the one that is just in the inventory list and not in use
	//suite.Assert().Fail("TODO implement me")
}

func (s *HandleDropSuite) TestDropAll() {
	s.get("all")
	s.Require().Equal(2, s.p.Inventory().Len())

	s.drop("all")

	// one event per thing that hit the floor
	sent[event.Dropped](s.T(), s.r, 2)
	sent[event.Dropped](s.T(), s.r, 3)
	s.Assert().Equal(0, s.p.Inventory().Len())
	s.Assert().Equal(2, s.w.StartRoom.Inventory.Len())
}

// "drop all" while wearing something drops the rest and keeps what you have on
func (s *HandleDropSuite) TestDropAllKeepsWhatIsWorn() {
	s.get("all")
	equip := command.Equip{Target: "knife", Slot: rules.SlotWield}
	s.w.handleEquip(s.handlerParameter(equip), equip)

	s.drop("all")

	s.Assert().Equal(1, s.p.Inventory().Len(), "still holding the knife")
	found := s.p.Inventory().FindAll("knife")
	s.Require().Len(found, 1)
	s.Assert().True(s.p.Equipment().ItemEquipped(found[0]))
	s.Assert().Equal(1, s.w.StartRoom.Inventory.Len())
}

// ...but if the only thing named is worn, saying nothing would be strange
func (s *HandleDropSuite) TestDropAllWhenEverythingIsWorn() {
	s.get("knife")
	equip := command.Equip{Target: "knife", Slot: rules.SlotWield}
	s.w.handleEquip(s.handlerParameter(equip), equip)
	s.r.Sent = nil

	s.drop("all")

	failed := sent[event.Failed](s.T(), s.r, 0)
	s.Assert().Equal("drop", failed.Verb)
	s.Assert().Equal(event.TargetInUse, failed.Code)
	s.Assert().Equal(1, s.p.Inventory().Len())
}

// the inventory is in pickup order, so "2.knife" means the second one you took
func (s *HandleDropSuite) TestDropNth() {
	s.get("knife")
	first := s.p.Inventory().FindAll("knife")[0]
	second := object.NewInstance(uuid.New(), first.Definition)
	s.p.Inventory().Add(second)

	s.drop("2.knife")

	_, stillHeld := s.p.Inventory().Get(second.Id)
	s.Assert().False(stillHeld, "should have dropped the second knife")
	_, firstHeld := s.p.Inventory().Get(first.Id)
	s.Assert().True(firstHeld, "the first one stays")
}

func (s *HandleDropSuite) TestDropUnparseableTarget() {
	s.drop("x.knife")

	failed := sent[event.Failed](s.T(), s.r, 0)
	s.Assert().Equal("drop", failed.Verb)
	s.Assert().Equal(event.ParseError, failed.Code)
}
