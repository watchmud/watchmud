package spaces

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

type occupancySuite struct {
	suite.Suite
	mob       *mobile.Instance
	player    *player.Player
	roomOne   *Room
	roomTwo   *Room
	occupancy *Occupancy
}

func TestOccupancySuite(t *testing.T) {
	suite.Run(t, new(occupancySuite))
}

func (s *occupancySuite) SetupTest() {
	d := mobile.NewDefinition(
		"id",
		"name",
		"zone",
		[]string{},
		"shortdesc",
		"roomdesc",
		25, rules.WanderDefinition{},
		10,
		false,
	)
	s.mob = mobile.NewInstance(d)

	s.player = player.NewTestPlayer(uuid.New(), "bob", nil)

	s.roomOne = NewTestRoom("one")
	s.roomTwo = NewTestRoom("two")

	s.occupancy = NewOccupancy()
}

func (s *occupancySuite) TestMovePlayer() {
	s.occupancy.PlacePlayer(s.player, s.roomOne)

	s.occupancy.MovePlayer(s.player, rules.DirectionEast, s.roomTwo)

	s.Assert().Equal(s.roomTwo, s.occupancy.RoomOfPlayer(s.player))
	s.Assert().Equal(0, len(s.roomOne.Players()))
	s.Assert().Equal(1, len(s.roomTwo.Players()))
	s.Assert().Equal(s.player, s.roomTwo.Players()[0])
}

func (s *occupancySuite) TestMoveUnplacedPlayer() {
	s.occupancy.MovePlayer(s.player, rules.DirectionEast, s.roomTwo)

	s.Assert().Equal(s.roomTwo, s.occupancy.RoomOfPlayer(s.player))
	s.Assert().Equal(0, len(s.roomOne.Players()))
	s.Assert().Equal(1, len(s.roomTwo.Players()))
	s.Assert().Equal(s.player, s.roomTwo.Players()[0])
}

func (s *occupancySuite) TestRemovePlayer() {
	s.occupancy.PlacePlayer(s.player, s.roomOne)
	s.occupancy.RemovePlayer(s.player)

	s.Assert().Equal(0, len(s.roomOne.Players()))
	s.Assert().Equal(0, len(s.roomTwo.Players()))
}

func (s *occupancySuite) TestMoveMobile() {
	s.occupancy.PlaceMobile(s.mob, s.roomOne)

	s.occupancy.MoveMobile(s.mob, rules.DirectionEast, s.roomTwo)

	s.Assert().Equal(s.roomTwo, s.occupancy.RoomOfMobile(s.mob))
	s.Assert().Equal(0, len(s.roomOne.Mobiles()))
	s.Assert().Equal(1, len(s.roomTwo.Mobiles()))
	s.Assert().Equal(s.mob, s.roomTwo.Mobiles()[0])
}

func (s *occupancySuite) newMob(name string) *mobile.Instance {
	return mobile.NewInstance(mobile.NewDefinition(name, name, "zone", nil,
		"shortdesc", "roomdesc", 25, rules.WanderDefinition{}, 10, false))
}

// The mobile pulse walks Mobiles(), so its order is the order mobs wander,
// aggro and regen in. It has to be the same every time: the order placed,
// untouched by moving, and closed up by a removal.
func (s *occupancySuite) TestMobilesInTheOrderPlaced() {
	rat, lizard, wolf := s.newMob("rat"), s.newMob("lizard"), s.newMob("wolf")
	s.occupancy.PlaceMobile(rat, s.roomOne)
	s.occupancy.PlaceMobile(lizard, s.roomTwo)
	s.occupancy.PlaceMobile(wolf, s.roomOne)

	s.occupancy.MoveMobile(rat, rules.DirectionEast, s.roomTwo)
	s.Assert().Equal([]*mobile.Instance{rat, lizard, wolf}, s.occupancy.Mobiles())

	s.occupancy.RemoveMobile(lizard)
	s.Assert().Equal([]*mobile.Instance{rat, wolf}, s.occupancy.Mobiles())
	s.Assert().Equal(1, s.occupancy.MobileCount("wolf"))
}
