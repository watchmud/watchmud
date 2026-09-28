package world

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
)

// What a mob leaves behind. testcontent's target drone (power 3) has a loot
// table of a knife at 50% and a rope at 100%. Each entry takes a d100 (0-99)
// for whether it drops, and each drop another for its power bump.
type lootSuite struct {
	worldTestSuite
	dice *testdice.LoadedDice
}

func TestLootSuite(t *testing.T) {
	suite.Run(t, new(lootSuite))
}

func (s *lootSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.dice = testdice.New()
	s.w.roller = s.dice
}

// kill the target drone and return its corpse
func (s *lootSuite) killDrone() *object.Instance {
	s.T().Helper()
	drone, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	s.w.combatantDied(drone, s.w.StartRoom)

	for item := range s.w.StartRoom.Inventory.All() {
		if item.Contents != nil {
			return item
		}
	}
	s.Require().Fail("no corpse")
	return nil
}

func (s *lootSuite) contents(corpse *object.Instance) map[string]int {
	got := map[string]int{}
	for item := range corpse.Contents.All() {
		got[item.Definition.ObjectId.DefinitionId] = item.Power
	}
	return got
}

// The drops go into the corpse, at the mob's power. Knife: 49 is under its
// 50, and a bump roll of 50 is nothing. Rope: 0 drops it, and a bump of 0 is +2.
func (s *lootSuite) TestDropsGoIntoTheCorpseAtTheMobsPower() {
	s.dice.Load([]int{49, 50, 0, 0})

	corpse := s.killDrone()

	s.Assert().Equal(map[string]int{"knife": 3, "rope": 3 + 2}, s.contents(corpse),
		"knife unbumped; rope rolled a 0 for the +2")
}

func (s *lootSuite) TestAMissedRollDropsNothing() {
	s.dice.Load([]int{50, 0, 99})

	corpse := s.killDrone()

	s.Assert().Equal(map[string]int{"rope": 3}, s.contents(corpse), "50 misses a 50% chance")
}

// Dice that fail are a bug, not a reason to lose the corpse.
func (s *lootSuite) TestNoDiceStillLeavesAnEmptyCorpse() {
	corpse := s.killDrone()
	s.Assert().Equal(0, corpse.Contents.Len())
}

// A corpse is a container, not a thing you carry off.
func (s *lootSuite) TestTheCorpseCantBeTaken() {
	s.dice.Load([]int{99, 99})
	s.killDrone()
	s.r.Sent = nil

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Get{Target: "corpse"})))

	s.Assert().Equal(event.TargetNotGettable, sent[event.Failed](s.T(), s.r, 0).Code)
}

// A corpse lasts rules.CorpseDecay, and then it's gone, loot and all, with a
// line to the room. Nothing else on the floor goes with it.
func (s *lootSuite) TestCorpsesDecay() {
	s.dice.Load([]int{99, 0, 99})
	corpse := s.killDrone()
	s.Require().Equal(1, corpse.Contents.Len())
	s.r.Sent = nil

	s.w.decayCorpses(time.Now())
	_, stillThere := s.w.StartRoom.Inventory.Get(corpse.Id)
	s.Assert().True(stillThere, "not yet")
	s.Assert().Empty(s.r.Sent)

	s.w.decayCorpses(time.Now().Add(rules.CorpseDecay + time.Second))
	_, stillThere = s.w.StartRoom.Inventory.Get(corpse.Id)
	s.Assert().False(stillThere, "gone")
	s.Assert().Equal("the corpse of Target Drone", sent[event.Decayed](s.T(), s.r, 0).Item)

	s.Assert().NotEmpty(s.w.StartRoom.Inventory.FindAll("knife"), "the room's own knife doesn't decay")
}
