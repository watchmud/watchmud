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

type junkSuite struct{ worldTestSuite }

func TestJunkSuite(t *testing.T) { suite.Run(t, new(junkSuite)) }

func (s *junkSuite) do(cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(cmd)))
}

func (s *junkSuite) carry(name string, spec *object.ContainerSpec) *object.Instance {
	s.T().Helper()
	d := object.NewDefinition(name, name, "wrathrock", rules.ObjectCategoryOther, nil,
		"a "+name, "A "+name+" is here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	d.Container = spec
	inst := object.NewInstance(uuid.New(), d)
	s.Require().NoError(s.p.Inventory().Add(inst))
	return inst
}

func (s *junkSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

// gone, and nothing back for it
func (s *junkSuite) TestJunk() {
	pelt := s.carry("pelt", nil)
	floor := s.w.StartRoom.Inventory.Len()

	s.do(command.Junk{Target: "pelt"})

	s.Assert().Equal(event.Junked{Actor: "testdood", Item: "a pelt"}, sent[event.Junked](s.T(), s.r, 0))
	_, still := s.p.Inventory().Get(pelt.Id)
	s.Assert().False(still)
	s.Assert().Equal(floor, s.w.StartRoom.Inventory.Len(), "not dropped: gone")
	s.Assert().Zero(s.p.Coins())
}

func (s *junkSuite) TestJunkRefusals() {
	bag := s.carry("satchel", &object.ContainerSpec{Portable: true})
	s.Require().NoError(object.Move(s.carry("pelt", nil), s.p.Inventory(), bag.Contents))
	s.do(command.Junk{Target: "satchel"})
	s.Assert().Equal(event.NotEmpty, s.failed(), "empty it first")

	s.do(command.Junk{Target: "dragon"})
	s.Assert().Equal(event.TargetNotFound, s.failed())
	s.do(command.Junk{})
	s.Assert().Equal(event.NoTarget, s.failed())
	s.do(command.Junk{Target: "5 coins"})
	s.Assert().Equal(event.CoinsInPurse, s.failed())
}

// from anywhere to the donation room's floor, bag and all, on the drop clock
func (s *junkSuite) TestDonate() {
	bag := s.carry("satchel", &object.ContainerSpec{Portable: true})
	s.Require().NoError(object.Move(s.carry("pelt", nil), s.p.Inventory(), bag.Contents))

	s.do(command.Donate{Target: "satchel"})

	donation, found := s.w.findRoomById("wrathrock", "donation_room")
	s.Require().True(found)
	got, there := donation.Inventory.Get(bag.Id)
	s.Require().True(there)
	s.Assert().Equal(1, got.Contents.Len(), "with what was in it")
	s.Assert().False(got.DecaysAt.IsZero(), "lying there like anything dropped")
	s.Assert().Equal(event.Donated{Actor: "testdood", Item: "a satchel"}, sent[event.Donated](s.T(), s.r, 0))
}

func (s *junkSuite) TestWornStaysOn() {
	helm := object.NewDefinition("helm", "helm", "wrathrock", rules.ObjectCategoryArmor, nil,
		"a helm", "A helm is here.", rules.SlotHead, rules.ArmorTypeLeather, nil)
	worn := object.NewInstance(uuid.New(), helm)
	s.Require().NoError(s.p.Inventory().Add(worn))
	s.p.Equipment().Equip(rules.SlotHead, worn)
	s.carry("pelt", nil)

	s.do(command.Junk{Target: "helm"})
	s.Assert().Equal(event.TargetInUse, s.failed())
	s.do(command.Donate{Target: "all"})
	s.Assert().Len(s.r.Sent, 1, "the pelt, not the helm")
	_, kept := s.p.Inventory().Get(worn.Id)
	s.Assert().True(kept)
}
