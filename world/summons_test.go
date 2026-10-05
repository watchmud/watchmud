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

// deaths and crumbles in the order the room heard them
func (s *summonsSuite) endings() []any {
	var out []any
	for _, m := range s.r.Sent {
		switch m.(type) {
		case event.Died, event.Crumbled:
			out = append(out, m)
		}
	}
	return out
}

// the summoner dies: his death, then his guard goes to dust
func (s *summonsSuite) TestSummonerDies() {
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)
	s.r.Clear()
	s.dice.Load([]int{99, 99, 0, 0}) // the drone's loot: no knife, the rope (always) and its bump; then coins

	s.w.combatantDied(s.drone, s.w.StartRoom)

	s.Assert().Empty(s.imps())
	s.Assert().Equal([]any{
		event.Died{Target: "Target Drone"},
		event.Crumbled{Name: "imp"},
		event.Crumbled{Name: "imp"},
	}, s.endings())
	s.Assert().False(s.w.fightLedger.InFight(s.p), "nothing left fighting")
}

// Review focus 2: a summon killed leaves nothing, and the rest stay
func (s *summonsSuite) TestKilledSummonLeavesNothing() {
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)
	first := s.imps()[0]
	s.r.Clear()

	s.w.combatantDied(first, s.w.StartRoom)

	s.Assert().Len(s.imps(), 1, "the other stays")
	s.Assert().Empty(s.w.StartRoom.Inventory.FindAll("corpse"), "no corpse, so no loot or coins")
	s.Assert().Equal([]any{event.Died{Target: "imp"}, event.Crumbled{Name: "imp"}}, s.endings())
}

// Review focus 3: the fight ends some other way -- here, everyone leaves it --
// and the next round's sweep crumbles them
func (s *summonsSuite) TestFightEndsOtherwise() {
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)
	s.w.fightLedger.EndAllFightsWith(s.p.Id())
	s.w.fightLedger.EndAllFightsWith(s.other.Id())

	s.w.DoViolence(10)

	s.Assert().Empty(s.imps())
}

// while the summoner fights, the sweep leaves them be
func (s *summonsSuite) TestSweepLeavesAFightGoingOn() {
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)

	s.w.sweepSummons()

	s.Assert().Len(s.imps(), 2)
}

// a summoner gone from the world without dying -- the sweep catches that too
func (s *summonsSuite) TestSummonerGone() {
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)
	s.w.fightLedger.EndAllFightsWith(s.drone.Id())
	s.w.RemoveMobile(s.drone)

	s.w.sweepSummons()

	s.Assert().Empty(s.imps())
}

// Review focus 1: DoViolence ranges over a snapshot of the fights, so one
// whose fighter left the world earlier in the round is still in it. It must
// not swing: no room to tell, and the damage would land from nowhere.
func (s *summonsSuite) TestRemovedFighterDoesntSwing() {
	s.dice.Load([]int{0})
	s.w.summon(s.drone, s.imp, 1)
	imp := s.imps()[0]
	s.w.occupancy.RemoveMobile(imp) // out of the world, its fight left behind
	s.Require().NotNil(s.w.fightLedger.GetFight(imp))
	before := s.p.CurrentHealth() + s.other.CurrentHealth()
	s.dice.Load([]int{20, 4}) // a hit, for 4

	s.w.DoViolence(10)

	s.Assert().Equal(before, s.p.CurrentHealth()+s.other.CurrentHealth())
}
