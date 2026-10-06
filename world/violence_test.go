package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
)

// What happens to everyone else's fights when one combatant dies. These drive
// the death handling directly, since that is where the bookkeeping lives --
// DoViolence itself rolls through w.roller and can be driven a swing at a
// time, which violence_armorclass_test.go does.
type violenceSuite struct {
	worldTestSuite
}

func TestViolenceSuite(t *testing.T) {
	suite.Run(t, new(violenceSuite))
}

// Two drones on one player. Killing one must leave the other one fighting --
// this ended every fight the winner was in, so a second attacker silently
// dropped out the moment the first one died.
func (s *violenceSuite) TestKillingOneAttackerLeavesTheOtherFighting() {
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	little, exists := s.w.StartRoom.FindMobile("little")
	s.Require().True(exists)

	s.Require().NoError(s.w.fightLedger.Fight(target, s.p))
	s.Require().NoError(s.w.fightLedger.Fight(little, s.p))
	s.Require().True(s.w.fightLedger.IsFighting(little))

	s.w.combatantDied(target, s.w.StartRoom)

	s.Assert().False(s.w.fightLedger.IsFighting(target), "the dead one stops fighting")
	s.Assert().True(s.w.fightLedger.IsFighting(little), "the survivor keeps fighting")
	s.Assert().True(s.w.fightLedger.InFight(s.p), "and the player is still in a fight")
}

// Your target dies while something else is still hitting you: you turn on
// it, rather than stand there "in a fight" and never swing again. The same
// rule is what turns the Barrow-King on the next player once the tank falls.
func (s *violenceSuite) TestWhenYourTargetDiesYouTurnOnTheNextAttacker() {
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	little, exists := s.w.StartRoom.FindMobile("little")
	s.Require().True(exists)

	s.Require().NoError(s.w.fightLedger.Fight(s.p, target))
	s.Require().NoError(s.w.fightLedger.Fight(little, s.p))

	s.w.combatantDied(target, s.w.StartRoom)

	fight := s.w.fightLedger.GetFight(s.p)
	s.Require().NotNil(fight, "the player is fighting again")
	s.Assert().Same(little, fight.Fightee)
}

// The one who died leaves in both directions: nothing is left swinging at a
// corpse either.
func (s *violenceSuite) TestTheDeadLeaveInBothDirections() {
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)

	s.Require().NoError(s.w.fightLedger.Fight(s.p, target))

	s.w.combatantDied(target, s.w.StartRoom)

	s.Assert().False(s.w.fightLedger.InFight(target))
	s.Assert().False(s.w.fightLedger.InFight(s.p))
}

func (s *violenceSuite) TestTheRoomIsTold() {
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)

	s.w.combatantDied(target, s.w.StartRoom)

	s.Assert().Equal("Target Drone", sent[event.Died](s.T(), s.r, 0).Target)
}

// A fight can outlive its room (the ledger snapshots zone and room ids when
// it starts), and then there is nobody to tell.
func (s *violenceSuite) TestNoRoomNoNotification() {
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)

	s.w.combatantDied(target, nil)

	s.Assert().Empty(s.r.Sent)
}

// A fight happens where the fighters are standing, not where it started.
// Nothing lets the fighter walk-off mid-flight, so a test has to carry
// them; this used to report the swing to temple_square forever, because
// the fight had written down where it began.
func (s *violenceSuite) TestTheFightIsWhereTheFightersAre() {
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	market, found := s.w.findRoomById("wrathrock", "market_square")
	s.Require().True(found)

	// one swing: the drone at the player, and a d20 of 1 so it misses and nothing else is rolled
	s.Require().NoError(s.w.fightLedger.Fight(target, s.p))
	s.w.fightLedger.EndFight(s.p)
	dice := testdice.New()
	dice.Load([]int{1})
	s.w.roller = dice

	// both of them are carried off to the market after the fight began
	s.w.moveMobile(target, rules.DirectionNone, market)
	s.w.movePlayer(s.p, rules.DirectionNone, market)

	templeEars := &player.Recorder{}
	s.w.PlacePlayer(player.NewTestPlayer(uuid.New(), "templewatcher", templeEars), s.w.StartRoom)
	marketEars := &player.Recorder{}
	s.w.PlacePlayer(player.NewTestPlayer(uuid.New(), "marketwatcher", marketEars), market)

	s.w.DoViolence(5)

	s.Assert().True(heardStruck(marketEars, target.Name()), "the market sees the swing")
	s.Assert().False(heardStruck(templeEars, target.Name()), "the temple doesn't")
}

func heardStruck(r *player.Recorder, attacker string) bool {
	for _, msg := range r.Sent {
		if struck, ok := msg.(event.Struck); ok && struck.Attacker == attacker {
			return true
		}
	}
	return false
}

// Killed by the first of two mobs and revived in the room they fell in -- the
// death room -- a player isn't swung at again by the second the same round:
// their fights ended with the death, whatever the round began with.
func (s *violenceSuite) TestNoSecondDeathInOneRound() {
	s.Require().Same(s.w.DeathRoom, s.w.StartRoom, "the test world dies where it starts")
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	little, exists := s.w.StartRoom.FindMobile("little")
	s.Require().True(exists)
	s.Require().NoError(s.w.fightLedger.Fight(target, s.p))
	s.Require().NoError(s.w.fightLedger.Fight(little, s.p))
	s.p.TakeMeleeDamage(s.p.CurrentHealth() - 1)
	dice := testdice.New()
	dice.Load([]int{20, 5, 20, 5, 0, 0, 0, 0, 0, 0, 0, 0})
	s.w.roller = dice
	s.r.Clear()

	s.w.DoViolence(1000)

	deaths := 0
	for _, m := range s.r.Sent {
		if d, ok := m.(event.Died); ok && d.IsPlayer {
			deaths++
		}
	}
	s.Assert().Equal(1, deaths)
}
