package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

type handleLookSuite struct {
	worldTestSuite
	other *player.Player
}

func TestHandleLookSuite(t *testing.T) {
	suite.Run(t, new(handleLookSuite))
}

func (s *handleLookSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.other = player.NewTestPlayer(uuid.New(), "other", nil)
	s.w.PlacePlayer(s.other, s.w.StartRoom)
}

func (s *handleLookSuite) TestSuccess() {
	cmd := command.Look{}
	s.w.handleLook(s.handlerParameter(cmd), cmd)

	desc := sent[event.RoomDescription](s.T(), s.r, 0)
	s.Assert().NotEmpty(desc.Name)
	s.Assert().NotEmpty(desc.Description)
	s.Assert().Equal(1, len(desc.Players))
	s.Assert().Equal("other", desc.Players[0])
}

func (s *handleLookSuite) look(target string) {
	s.T().Helper()
	s.r.Clear()
	cmd := command.Look{Target: target}
	s.w.handleLook(s.handlerParameter(cmd), cmd)
}

// a thing carried: what it is, and whether you have it on
func (s *handleLookSuite) TestAtSomethingCarried() {
	d := object.NewTestDefinition(s.T(), "hood", rules.SlotHead, rules.ObjectCategoryArmor, rules.ArmorTypeLeather)
	d.Abilities = []string{"assess"}
	d.MaxDurability = 20
	hood := object.NewInstance(uuid.New(), d)
	hood.Power, hood.Durability = 3, 12
	s.Require().NoError(s.p.Inventory().Add(hood))
	s.p.Equipment().Equip(rules.SlotHead, hood)

	s.look("hood")

	got := sent[event.LookedAtObject](s.T(), s.r, 0)
	s.Assert().True(got.Worn)
	s.Assert().Equal(3, got.Power)
	s.Assert().Equal(12, got.Durability)
	s.Assert().Equal(rules.ArmorTypeLeather, got.ArmorType)
	s.Assert().Positive(got.Armor, "leather on the head adds something")
	s.Assert().Equal([]string{"assess"}, got.Abilities)
}

// a bag on the floor: how full it is
func (s *handleLookSuite) TestAtABagOnTheFloor() {
	d := object.NewDefinition("satchel", "satchel", "wrathrock", rules.ObjectCategoryOther, nil,
		"a satchel", "A satchel is here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	d.Container = &object.ContainerSpec{Portable: true, Capacity: 10}
	s.Require().NoError(s.w.StartRoom.Inventory.Add(object.NewInstance(uuid.New(), d)))

	s.look("satchel")

	got := sent[event.LookedAtObject](s.T(), s.r, 0)
	s.Assert().True(got.Container)
	s.Assert().Equal(10, got.Capacity)
	s.Assert().False(got.Worn)
}

// a mob: how it seems, not its numbers
func (s *handleLookSuite) TestAtAMob() {
	mob, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	mob.CurHealth = mob.Definition.MaxHealth / 4

	s.look("target")

	got := sent[event.LookedAtMob](s.T(), s.r, 0)
	s.Assert().Equal(mob.Definition.ShortDescription, got.Name)
	s.Assert().Equal(percent(mob.CurHealth, mob.Definition.MaxHealth), got.Health)
	s.Assert().Less(got.Health, 50)
}

func (s *handleLookSuite) TestAtAPlayer() {
	s.look("OTHER")
	got := sent[event.LookedAtPlayer](s.T(), s.r, 0)
	s.Assert().Equal("other", got.Name)
	s.Assert().Equal(100, got.Health)
}

func (s *handleLookSuite) TestAtNothing() {
	s.look("dragon")
	s.Assert().Equal(event.TargetNotFound, sent[event.Failed](s.T(), s.r, 0).Code)
}
