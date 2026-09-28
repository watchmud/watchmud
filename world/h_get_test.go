package world

import (
	"slices"
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/behavior"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type HandleGetSuite struct {
	worldTestSuite
}

func TestHandleGetSuite(t *testing.T) {
	suite.Run(t, new(HandleGetSuite))
}

func (s *HandleGetSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
}

func (s *HandleGetSuite) TestSuccess() {
	// start off with two items in the room and zero in the player
	s.Assert().Equal(2, s.w.StartRoom.Inventory.Len())
	s.Assert().Equal(0, s.p.Inventory().Len())

	cmd := command.Get{Target: "knife"}
	s.w.handleGet(s.handlerParameter(cmd), cmd)

	s.Assert().Equal(1, len(s.r.Sent))
	got := sent[event.Got](s.T(), s.r, 0)
	s.Assert().Equal("testdood", got.Actor)

	// player has one item
	s.Assert().Equal(1, s.p.Inventory().Len())
	found := s.p.Inventory().FindAll("knife")
	s.Assert().True(len(found) > 0)
	s.Assert().Equal("knife", found[0].Definition.Name)

	// there's one other item in the room now
	s.Assert().Equal(1, s.w.StartRoom.Inventory.Len())
}

func (s *HandleGetSuite) TestAliasTarget() {
	cmd := command.Get{Target: "iron"}
	s.w.handleGet(s.handlerParameter(cmd), cmd)

	s.Assert().Equal(1, len(s.r.Sent))
	sent[event.Got](s.T(), s.r, 0)
	s.Assert().Equal(1, s.p.Inventory().Len())

	found := s.p.Inventory().FindAll("helmet")
	s.Assert().True(len(found) > 0)
	s.Assert().Equal("iron helmet", found[0].Definition.Name)
	s.Assert().Equal(1, s.w.StartRoom.Inventory.Len())
}

func (s *HandleGetSuite) TestTargetNotInRoom() {
	cmd := command.Get{Target: "bag_of_coins"}
	s.w.handleGet(s.handlerParameter(cmd), cmd)

	s.Assert().Equal(1, len(s.r.Sent))
	failed := sent[event.Failed](s.T(), s.r, 0)
	s.Assert().Equal("get", failed.Verb)
	s.Assert().Equal(event.TargetNotFound, failed.Code)

	// player has zero items still
	s.Assert().Equal(0, s.p.Inventory().Len())

	// still two items in start room
	s.Assert().Equal(2, s.w.StartRoom.Inventory.Len())
}

func (s *HandleGetSuite) TestNoTarget() {
	cmd := command.Get{Target: ""}
	s.w.handleGet(s.handlerParameter(cmd), cmd)

	s.Assert().Equal(1, len(s.r.Sent))
	failed := sent[event.Failed](s.T(), s.r, 0)
	s.Assert().Equal(event.NoTarget, failed.Code)

	// player has zero items, start room still has 2
	s.Assert().Equal(0, s.p.Inventory().Len())
	s.Assert().Equal(2, s.w.StartRoom.Inventory.Len())
}

// another knife on the floor, so "2.knife" and "all.knife" have something to
// disagree about
func (s *HandleGetSuite) addKnife() *object.Instance {
	s.T().Helper()
	d := object.NewDefinition(
		"knife",
		"knife",
		"wrathrock",
		rules.ObjectCategoryWeapon,
		nil,
		"second knife",
		"Another knife is here.",
		rules.SlotWield,
		rules.ArmorTypeNone)
	inst := object.NewInstance(uuid.New(), d)
	s.Require().NoError(s.w.StartRoom.Inventory.Add(inst))
	return inst
}

// something bolted down
func (s *HandleGetSuite) addFountain() *object.Instance {
	s.T().Helper()
	d := object.NewDefinition(
		"fountain",
		"fountain",
		"wrathrock",
		rules.ObjectCategoryOther,
		nil,
		"fountain",
		"A fountain bubbles here.",
		rules.SlotNone,
		rules.ArmorTypeNone,
	)
	d.Behaviors.Add(behavior.NoTake)
	inst := object.NewInstance(uuid.New(), d)
	s.Require().NoError(s.w.StartRoom.Inventory.Add(inst))
	return inst
}

func (s *HandleGetSuite) get(target string) {
	s.T().Helper()
	cmd := command.Get{Target: target}
	s.w.handleGet(s.handlerParameter(cmd), cmd)
}

func (s *HandleGetSuite) TestGetAll() {
	s.get("all")

	// one event per thing picked up
	s.Assert().Equal(2, len(s.r.Sent))
	sent[event.Got](s.T(), s.r, 0)
	sent[event.Got](s.T(), s.r, 1)

	s.Assert().Equal(2, s.p.Inventory().Len())
	s.Assert().Equal(0, s.w.StartRoom.Inventory.Len())
}

func (s *HandleGetSuite) TestGetAllOfOneName() {
	s.addKnife()

	s.get("all.knife")

	s.Assert().Equal(2, len(s.r.Sent))
	s.Assert().Equal(2, s.p.Inventory().Len())
	// the helmet is still on the floor
	s.Assert().Equal(1, s.w.StartRoom.Inventory.Len())
}

// "2.knife" is the second one that landed there, not whichever comes to hand
func (s *HandleGetSuite) TestGetNth() {
	second := s.addKnife()

	s.get("2.knife")

	s.Assert().Equal(1, s.p.Inventory().Len())
	_, held := s.p.Inventory().Get(second.Id)
	s.Assert().True(held, "should have taken the second knife")
}

func (s *HandleGetSuite) TestGetNthPastTheEnd() {
	s.get("2.knife")

	failed := sent[event.Failed](s.T(), s.r, 0)
	s.Assert().Equal("get", failed.Verb)
	s.Assert().Equal(event.TargetNotFound, failed.Code)
	s.Assert().Equal(0, s.p.Inventory().Len())
}

func (s *HandleGetSuite) TestGetSomethingBoltedDown() {
	s.addFountain()

	s.get("fountain")

	failed := sent[event.Failed](s.T(), s.r, 0)
	s.Assert().Equal(event.TargetNotGettable, failed.Code)
	s.Assert().Equal(0, s.p.Inventory().Len())
}

// "get all" in a room with a fountain in it takes everything else and says
// nothing about the fountain
func (s *HandleGetSuite) TestGetAllLeavesWhatIsBoltedDown() {
	fountain := s.addFountain()

	s.get("all")

	s.Assert().Equal(2, len(s.r.Sent))
	s.Assert().Equal(2, s.p.Inventory().Len())
	s.Assert().Equal(1, s.w.StartRoom.Inventory.Len())
	_, stillThere := s.w.StartRoom.Inventory.Get(fountain.Id)
	s.Assert().True(stillThere)
}

// ...but if the fountain is all there is, saying nothing would be strange
func (s *HandleGetSuite) TestGetAllWhenNothingCanBeTaken() {
	// snapshot first: removing while ranging the live contents is asking for it
	for _, inst := range slices.Collect(s.w.StartRoom.Inventory.All()) {
		s.Require().NoError(s.w.StartRoom.Inventory.Remove(inst))
	}
	s.addFountain()

	s.get("all")

	failed := sent[event.Failed](s.T(), s.r, 0)
	s.Assert().Equal(event.TargetNotGettable, failed.Code)
}

// the parse error is ours, not the player's: they get one message about it
func (s *HandleGetSuite) TestGetUnparseableTarget() {
	s.get("x.knife")

	failed := sent[event.Failed](s.T(), s.r, 0)
	s.Assert().Equal("get", failed.Verb)
	s.Assert().Equal(event.ParseError, failed.Code)
	s.Assert().Equal(0, s.p.Inventory().Len())
}
