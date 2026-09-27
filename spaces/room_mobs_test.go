package spaces

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/rules"
)

type RoomMobsSuite struct {
	suite.Suite
	room   *Room
	lizard *mobile.Definition
	rat    *mobile.Definition
}

func TestRoomMobsSuite(t *testing.T) {
	suite.Run(t, new(RoomMobsSuite))
}

func (s *RoomMobsSuite) SetupTest() {
	s.room = NewTestRoom("foo")
	s.lizard = newMobDefinition("lizard", []string{"scaly"})
	s.rat = newMobDefinition("rat", nil)
}

func newMobDefinition(name string, aliases []string) *mobile.Definition {
	return mobile.NewDefinition(
		name,
		name,
		"zone",
		aliases,
		"shortdesc",
		"roomdesc",
		25,
		rules.WanderDefinition{},
		10,
		false,
	)
}

// all returns the mobs in the order All() yields them.
func (s *RoomMobsSuite) all() []*mobile.Instance {
	return slices.Collect(s.room.mobs.All())
}

func (s *RoomMobsSuite) TestAddKeepsInsertionOrder() {
	one := mobile.NewInstance(s.lizard)
	two := mobile.NewInstance(s.lizard)
	three := mobile.NewInstance(s.rat)

	s.Require().NoError(s.room.mobs.Add(one))
	s.Require().NoError(s.room.mobs.Add(two))
	s.Require().NoError(s.room.mobs.Add(three))

	s.Equal([]*mobile.Instance{one, two, three}, s.all())
}

// mob1 added, mob2 added, mob1 leaves, mob3 added -> [mob2, mob3]
func (s *RoomMobsSuite) TestRemoveFromMiddleKeepsOrder() {
	one := mobile.NewInstance(s.lizard)
	two := mobile.NewInstance(s.lizard)
	three := mobile.NewInstance(s.rat)

	s.Require().NoError(s.room.mobs.Add(one))
	s.Require().NoError(s.room.mobs.Add(two))
	s.Require().NoError(s.room.mobs.Remove(one))
	s.Require().NoError(s.room.mobs.Add(three))

	s.Equal([]*mobile.Instance{two, three}, s.all())
}

func (s *RoomMobsSuite) TestAddDuplicateIsError() {
	one := mobile.NewInstance(s.lizard)
	s.Require().NoError(s.room.mobs.Add(one))

	s.Error(s.room.mobs.Add(one))
	s.Equal([]*mobile.Instance{one}, s.all())
}

func (s *RoomMobsSuite) TestRemoveMissingIsError() {
	one := mobile.NewInstance(s.lizard)

	s.Error(s.room.mobs.Remove(one))
	s.Empty(s.all())
}

func (s *RoomMobsSuite) TestRemoveEveryMob() {
	one := mobile.NewInstance(s.lizard)
	two := mobile.NewInstance(s.rat)
	s.Require().NoError(s.room.mobs.Add(one))
	s.Require().NoError(s.room.mobs.Add(two))

	s.Require().NoError(s.room.mobs.Remove(two))
	s.Require().NoError(s.room.mobs.Remove(one))

	s.Empty(s.all())
}

// Find returns the earliest-added match, not whichever the map felt like.
func (s *RoomMobsSuite) TestFindReturnsFirstAdded() {
	one := mobile.NewInstance(s.lizard)
	two := mobile.NewInstance(s.lizard)
	s.Require().NoError(s.room.mobs.Add(one))
	s.Require().NoError(s.room.mobs.Add(two))

	found, exists := s.room.FindMobile("lizard")
	s.Require().True(exists)
	s.Same(one, found)

	// and once the first one leaves, the next in line answers
	s.Require().NoError(s.room.mobs.Remove(one))
	found, exists = s.room.FindMobile("lizard")
	s.Require().True(exists)
	s.Same(two, found)
}

func (s *RoomMobsSuite) TestFindByAlias() {
	one := mobile.NewInstance(s.lizard)
	s.Require().NoError(s.room.mobs.Add(one))

	found, exists := s.room.FindMobile("scaly")
	s.Require().True(exists)
	s.Same(one, found)
}

func (s *RoomMobsSuite) TestFindNotFound() {
	s.Require().NoError(s.room.mobs.Add(mobile.NewInstance(s.lizard)))

	found, exists := s.room.FindMobile("goblin")
	s.False(exists)
	s.Nil(found)
}
