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

// testdood gives to bob, both in Temple Square.
type handleGiveSuite struct {
	worldTestSuite
	bob    *player.Player
	bobRec *player.Recorder
}

func TestHandleGiveSuite(t *testing.T) {
	suite.Run(t, new(handleGiveSuite))
}

func (s *handleGiveSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.bobRec = &player.Recorder{}
	s.bob = player.NewTestPlayer(uuid.New(), "bob", s.bobRec)
	s.w.PlacePlayer(s.bob, s.w.StartRoom)
}

func (s *handleGiveSuite) give(target, to string) {
	s.T().Helper()
	s.r.Clear()
	s.bobRec.Clear()
	s.w.HandleIncomingMessage(s.handlerParameter(command.Give{Target: target, To: to}))
}

func (s *handleGiveSuite) carry(name string, spec *object.ContainerSpec) *object.Instance {
	s.T().Helper()
	d := object.NewDefinition(name, name, "wrathrock", rules.ObjectCategoryOther, nil,
		"a "+name, "A "+name+" is here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	d.Container = spec
	inst := object.NewInstance(uuid.New(), d)
	s.Require().NoError(s.p.Inventory().Add(inst))
	return inst
}

func (s *handleGiveSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

func (s *handleGiveSuite) TestGive() {
	pelt := s.carry("pelt", nil)

	s.give("pelt", "bob")

	want := event.Gave{Actor: "testdood", Recipient: "bob", Item: "a pelt"}
	s.Assert().Equal(want, sent[event.Gave](s.T(), s.r, 0))
	s.Assert().Equal(want, sent[event.Gave](s.T(), s.bobRec, 0))
	_, has := s.bob.Inventory().Get(pelt.Id)
	s.Assert().True(has)
	_, kept := s.p.Inventory().Get(pelt.Id)
	s.Assert().False(kept)
}

func (s *handleGiveSuite) TestGive_aBagGoesWithWhatsInIt() {
	bag := s.carry("satchel", &object.ContainerSpec{Portable: true})
	s.Require().NoError(object.Move(s.carry("pelt", nil), s.p.Inventory(), bag.Contents))

	s.give("satchel", "BOB")

	got, has := s.bob.Inventory().Get(bag.Id)
	s.Require().True(has)
	s.Assert().Equal(1, got.Contents.Len())
}

func (s *handleGiveSuite) TestGive_coins() {
	s.p.AddCoins(30)

	s.give("20 coins", "bob")
	s.Assert().Equal(event.Gave{Actor: "testdood", Recipient: "bob", Item: "20 coins"}, sent[event.Gave](s.T(), s.bobRec, 0))
	s.Assert().Equal(10, s.p.Coins())
	s.Assert().Equal(20, s.bob.Coins())

	s.give("50 coins", "bob")
	s.Assert().Equal(event.NotEnoughCoins, s.failed())
	s.Assert().Equal(10, s.p.Coins())
}

func (s *handleGiveSuite) TestGive_allSkipsWhatsWorn() {
	s.carry("pelt", nil)
	helm := object.NewDefinition("helm", "helm", "wrathrock", rules.ObjectCategoryArmor, nil,
		"a helm", "A helm is here.", rules.SlotHead, rules.ArmorTypeLeather, nil)
	worn := object.NewInstance(uuid.New(), helm)
	s.Require().NoError(s.p.Inventory().Add(worn))
	s.p.Equipment().Equip(rules.SlotHead, worn)

	s.give("helm", "bob")
	s.Assert().Equal(event.TargetInUse, s.failed())

	s.give("all", "bob")
	s.Assert().Len(s.bobRec.Sent, 1, "the pelt, and the helm stays on")
	s.Assert().Equal(1, s.bob.Inventory().Len())
}

func (s *handleGiveSuite) TestGive_refusals() {
	s.carry("pelt", nil)

	s.give("", "bob")
	s.Assert().Equal(event.NoTarget, s.failed())
	s.give("pelt", "")
	s.Assert().Equal(event.NoRecipient, s.failed())
	s.give("pelt", "alice")
	s.Assert().Equal(event.ToPlayerNotFound, s.failed())
	s.give("pelt", "testdood")
	s.Assert().Equal(event.GiveSelf, s.failed())
	s.give("dragon", "bob")
	s.Assert().Equal(event.TargetNotFound, s.failed())
}

// someone in another room is out of reach
func (s *handleGiveSuite) TestGive_onlyInTheRoom() {
	s.carry("pelt", nil)
	store, found := s.w.findRoomById("wrathrock", "general_store")
	s.Require().True(found)
	s.w.movePlayerMagically(s.bob, store)

	s.give("pelt", "bob")
	s.Assert().Equal(event.ToPlayerNotFound, s.failed())
}
