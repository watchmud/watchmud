package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
)

type mobFleeSuite struct {
	worldTestSuite
	mob  *mobile.Instance
	dice *testdice.LoadedDice
}

func TestMobFleeSuite(t *testing.T) { suite.Run(t, new(mobFleeSuite)) }

func (s *mobFleeSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	var found bool
	s.mob, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.dice = testdice.New()
	s.w.roller = s.dice
}

// it breaks off, runs through an open exit in its zone, and the room sees it go
func (s *mobFleeSuite) TestFlees() {
	s.Require().NoError(s.w.startFight(s.p, s.mob))
	s.r.Clear()
	s.dice.Load([]int{0})

	s.Assert().True(s.w.mobFlees(s.mob))

	s.Assert().NotSame(s.w.StartRoom, s.w.mobileRoom(s.mob))
	s.Assert().Same(s.w.StartRoom.Zone, s.w.mobileRoom(s.mob).Zone)
	s.Assert().False(s.w.fightLedger.InFight(s.mob))
	s.Assert().False(s.w.fightLedger.InFight(s.p), "nobody left to fight")
	s.Assert().Equal(event.Fled{Who: s.mob.Name()}, sent[event.Fled](s.T(), s.r, 0))
}

// not fighting, nothing to flee
func (s *mobFleeSuite) TestOnlyFromAFight() {
	s.Assert().False(s.w.mobFlees(s.mob))
	s.Assert().Same(s.w.StartRoom, s.w.mobileRoom(s.mob))
}

// A fight whose sides are in different rooms -- a mob fled earlier in the
// round, its fights still in that round's snapshot -- is skipped: nobody
// swings after it. Moved here with the fight left on the books, to see it.
func (s *mobFleeSuite) TestNoSwingAcrossRooms() {
	s.Require().NoError(s.w.startFight(s.p, s.mob))
	market, found := s.w.findRoomById("wrathrock", "market_square")
	s.Require().True(found)
	s.w.moveMobile(s.mob, rules.DirectionSouth, market)
	s.r.Clear()
	before := s.mob.CurHealth
	s.dice.Load([]int{20, 3, 20, 3}) // every swing would land, were it allowed

	s.w.DoViolence(1000)

	for _, m := range s.r.Sent {
		_, struck := m.(event.Struck)
		s.Assert().False(struck, "a swing across rooms")
	}
	s.Assert().Equal(before, s.mob.CurHealth)
}
