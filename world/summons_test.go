package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/testdice"
)

// The Target Drone stands in for a summoner, calling up testcontent's imps in
// temple square, where the player and a second player are standing.
type summonsSuite struct {
	worldTestSuite
	drone *mobile.Instance
	imp   *mobile.Definition
	other *player.Player
	dice  *testdice.LoadedDice
}

func TestSummonsSuite(t *testing.T) {
	suite.Run(t, new(summonsSuite))
}

func (s *summonsSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	var found bool
	s.drone, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.imp = s.w.Zone("wrathrock").MobileDefinitions["imp"]
	s.Require().NotNil(s.imp)
	s.other = player.NewTestPlayer(uuid.New(), "otherdood", &player.Recorder{})
	s.w.PlacePlayer(s.other, s.w.StartRoom)
	s.dice = testdice.New()
	s.w.roller = s.dice
	s.r.Clear()
}

// imps is every imp in temple square, in the order they arrived.
func (s *summonsSuite) imps() []*mobile.Instance {
	var imps []*mobile.Instance
	for _, m := range s.w.StartRoom.Mobiles() {
		if m.Definition == s.imp {
			imps = append(imps, m)
		}
	}
	return imps
}

func (s *summonsSuite) TestArrive() {
	s.dice.Load([]int{0, 0})

	got := s.w.summon(s.drone, s.imp, 2)

	s.Assert().Equal(2, got)
	imps := s.imps()
	s.Require().Len(imps, 2)
	for _, imp := range imps {
		s.Assert().Same(s.drone, imp.Summoner)
	}
	s.Assert().Equal(event.Summoned{Summoner: "Target Drone", Name: "imp", Count: 2},
		sent[event.Summoned](s.T(), s.r, 0))
	s.Assert().Equal(2, s.w.liveSummons(s.drone))
}

// each picks a player at random: the dice say otherdood, then testdood
func (s *summonsSuite) TestEachGoesForAPlayer() {
	s.dice.Load([]int{1, 0})

	s.w.summon(s.drone, s.imp, 2)

	imps := s.imps()
	s.Require().Len(imps, 2)
	s.Assert().Same(s.other, s.w.fightLedger.GetFight(imps[0]).Fightee)
	s.Assert().Same(s.p, s.w.fightLedger.GetFight(imps[1]).Fightee)
}

// a wizard with nohassle isn't on the list, so both dice pick otherdood
func (s *summonsSuite) TestSkipsANoHassleWizard() {
	s.p.SetWizard(true)
	s.p.SetNoHassle(true)
	s.dice.Load([]int{0, 0})

	s.w.summon(s.drone, s.imp, 2)

	for _, imp := range s.imps() {
		s.Assert().Same(s.other, s.w.fightLedger.GetFight(imp).Fightee)
	}
}

// Review focus 5: nobody to pick -- they arrive and stand
func (s *summonsSuite) TestNobodyToPick() {
	s.w.RemovePlayer(s.other)
	s.p.SetWizard(true)
	s.p.SetNoHassle(true)

	s.Assert().Equal(2, s.w.summon(s.drone, s.imp, 2))

	for _, imp := range s.imps() {
		s.Assert().False(s.w.fightLedger.InFight(imp))
	}
}

// a summon isn't the zone's: it mustn't keep a reset from refilling a room
func (s *summonsSuite) TestNotCountedForResets() {
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)

	s.Assert().Equal(0, s.w.occupancy.MobileCount("imp"))
}
