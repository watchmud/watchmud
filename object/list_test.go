package object

import (
	"slices"
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/ordered"
	"github.com/watchmud/watchmud/rules"
)

type ListSuite struct {
	suite.Suite
	list   *List
	first  *Instance
	second *Instance
}

func TestListSuite(t *testing.T) {
	suite.Run(t, new(ListSuite))
}

func (s *ListSuite) SetupTest() {
	s.list = NewList()

	s.first = s.instance("knife")
	s.second = s.instance("knife")

	s.Require().NoError(s.list.Add(s.first))
	s.Require().NoError(s.list.Add(s.second))
}

func (s *ListSuite) instance(id string) *Instance {
	d := NewTestDefinition(s.T(),
		id,
		rules.SlotNone,
		rules.ObjectCategoryOther,
		rules.ArmorTypeNone,
	)
	return NewInstance(uuid.New(), d)
}

func (s *ListSuite) TestRoomInventory_Remove() {
	s.Assert().NoError(s.list.Remove(s.first))

	s.Assert().Equal(1, s.list.Len())
}

// The same guarantee RoomMobs makes: taking something off the floor doesn't
// disturb the order of what's left, or of what lands there afterwards.
func (s *ListSuite) TestRoomInventory_RemoveKeepsOrder() {
	third := s.instance("knife")

	s.Require().NoError(s.list.Remove(s.first))
	s.Require().NoError(s.list.Add(third))

	s.Assert().Equal([]*Instance{s.second, third}, slices.Collect(s.list.All()))
}

func (s *ListSuite) TestMove() {
	from, to := NewList(), NewList()
	knife := s.instance("knife")
	s.Require().NoError(from.Add(knife))

	s.Require().NoError(Move(knife, from, to))

	s.Assert().Equal(0, from.Len())
	_, there := to.Get(knife.Id)
	s.Assert().True(there)
}

// Moving something that isn't there touches nothing.
func (s *ListSuite) TestMoveMissing() {
	from, to := NewList(), NewList()
	err := Move(s.instance("knife"), from, to)
	s.Assert().ErrorIs(err, ordered.ErrNotFound)
	s.Assert().Equal(0, to.Len())
}

// If it can't go in, it stays where it was: nothing is lost.
func (s *ListSuite) TestMoveRefusedPutsItBack() {
	from, to := NewList(), NewList()
	knife := s.instance("knife")
	s.Require().NoError(from.Add(knife))
	s.Require().NoError(to.Add(knife)) // a bug: it's in both

	s.Assert().ErrorIs(Move(knife, from, to), ordered.ErrDuplicate)
	_, stillThere := from.Get(knife.Id)
	s.Assert().True(stillThere)
}

// A floor holds its newest arrival first, however it got there: added
// directly or moved in. "look" lists it in that order, and bare "knife" (or
// "corpse") is the one that landed last.
func (s *ListSuite) TestFloorIsNewestFirst() {
	floor, carried := NewFloor(), NewList()
	first, second, dropped := s.instance("knife"), s.instance("knife"), s.instance("knife")
	s.Require().NoError(floor.Add(first))
	s.Require().NoError(floor.Add(second))
	s.Require().NoError(carried.Add(dropped))

	s.Require().NoError(Move(dropped, carried, floor))

	s.Assert().Equal([]*Instance{dropped, second, first}, slices.Collect(floor.All()))
	s.Assert().Same(dropped, floor.FindAll("knife")[0])
}
