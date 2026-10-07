package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/lock"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

// A satchel carried, with room for two things.
type bagSuite struct {
	tradeSuite
	bag *object.Instance
}

func TestBagSuite(t *testing.T) {
	suite.Run(t, new(bagSuite))
}

func (s *bagSuite) SetupTest() {
	s.tradeSuite.SetupTest()
	s.bag = s.item("satchel", []string{"bag"}, &object.ContainerSpec{Portable: true, Capacity: 2})
	s.Require().NoError(s.p.Inventory().Add(s.bag))
	s.r.Clear()
}

func (s *bagSuite) item(name string, aliases []string, spec *object.ContainerSpec) *object.Instance {
	s.T().Helper()
	d := object.NewDefinition(name, name, "wrathrock", rules.ObjectCategoryOther, aliases,
		"a "+name, "A "+name+" is here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	d.Container = spec
	return object.NewInstance(uuid.New(), d)
}

func (s *bagSuite) pocket(name string) *object.Instance {
	s.T().Helper()
	inst := s.item(name, nil, nil)
	s.Require().NoError(s.p.Inventory().Add(inst))
	return inst
}

func (s *bagSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

func (s *bagSuite) send(cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.do(cmd)
}

func (s *bagSuite) TestPutGetLook() {
	pelt := s.pocket("pelt")

	s.send(command.Put{Target: "pelt", Into: "bag"})
	s.Assert().Equal(event.Put{Actor: "testdood", Item: "a pelt", Into: "a satchel"}, sent[event.Put](s.T(), s.r, 0))
	_, inBag := s.bag.Contents.Get(pelt.Id)
	s.Assert().True(inBag)

	s.send(command.Look{Target: "bag", In: true})
	s.Assert().Len(sent[event.ContainerContents](s.T(), s.r, 0).Items, 1)

	s.send(command.Get{Target: "pelt", From: "bag"})
	sent[event.Got](s.T(), s.r, 0)
	_, carried := s.p.Inventory().Get(pelt.Id)
	s.Assert().True(carried)
}

// a bag has no lid: there's nothing to open or lock
func (s *bagSuite) TestNoLid() {
	s.Assert().Nil(s.bag.Lock)
	s.send(command.Open{Target: "bag"})
	s.Assert().Equal(event.NoDoor, s.failed())
}

// the one carried is found before one on the floor
func (s *bagSuite) TestCarriedFirst() {
	floor := s.item("satchel", []string{"bag"}, &object.ContainerSpec{Portable: true})
	s.Require().NoError(s.w.StartRoom.Inventory.Add(floor))
	s.pocket("pelt")

	s.send(command.Put{Target: "pelt", Into: "bag"})
	s.Assert().Equal(1, s.bag.Contents.Len())
	s.Assert().Zero(floor.Contents.Len())
}

func (s *bagSuite) TestFull() {
	s.pocket("pelt")
	s.pocket("pelt")
	s.pocket("pelt")

	s.send(command.Put{Target: "all.pelt", Into: "bag"})
	s.Assert().Len(s.r.Sent, 2, "two fit, and the third stays out")
	s.Assert().Equal(2, s.bag.Contents.Len())

	s.send(command.Put{Target: "pelt", Into: "bag"})
	s.Assert().Equal(event.ContainerFull, s.failed())
}

// nothing that holds things goes in a bag, itself least of all
func (s *bagSuite) TestNoNesting() {
	other := s.item("sack", nil, &object.ContainerSpec{Portable: true})
	s.Require().NoError(s.p.Inventory().Add(other))

	s.send(command.Put{Target: "sack", Into: "bag"})
	s.Assert().Equal(event.CantNest, s.failed())
	s.send(command.Put{Target: "satchel", Into: "bag"})
	s.Assert().Equal(event.CantNest, s.failed())

	s.pocket("pelt")
	s.send(command.Put{Target: "all", Into: "bag"})
	s.Assert().Len(s.r.Sent, 1, "the pelt, and neither bag")
	s.Assert().Equal(1, s.bag.Contents.Len())

	s.send(command.Put{Target: "all", Into: "bag"})
	s.Assert().Equal(event.CantNest, s.failed(), "only bags left to put")
}

func (s *bagSuite) TestCoinsStayInThePurse() {
	s.p.AddCoins(10)
	s.send(command.Put{Target: "5 coins", Into: "bag"})
	s.Assert().Equal(event.CoinsInPurse, s.failed())
	s.Assert().Equal(10, s.p.Coins())
}

// a key in a bag still opens what it opens
func (s *bagSuite) TestKeyInABag() {
	key := s.item("box_key", []string{"key"}, nil)
	s.Require().NoError(s.bag.Contents.Add(key))
	box := s.item("strongbox", []string{"box"}, &object.ContainerSpec{Initial: lock.State{Closed: true, Locked: true}, Key: "wrathrock/box_key"})
	s.Require().NoError(s.w.StartRoom.Inventory.Add(box))

	s.send(command.Unlock{Target: "box"})
	sent[event.ContainerChanged](s.T(), s.r, 0)
	s.Assert().False(box.Lock.Locked)
}

// what's in a bag would be sold with it
func (s *bagSuite) TestSellOnlyEmpty() {
	s.toStore()
	s.bag.Power = 1
	pelt := s.pocket("pelt")
	s.Require().NoError(object.Move(pelt, s.p.Inventory(), s.bag.Contents))

	s.send(command.Sell{Target: "bag"})
	s.Assert().Equal(event.NotEmpty, s.failed())

	s.Require().NoError(object.Move(pelt, s.bag.Contents, s.p.Inventory()))
	s.send(command.Sell{Target: "bag"})
	sent[event.Sold](s.T(), s.r, 0)
}
