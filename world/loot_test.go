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

// What a mob leaves behind. testcontent target drone (power 3) has a loot
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

	s.w.decayFloors(time.Now())
	_, stillThere := s.w.StartRoom.Inventory.Get(corpse.Id)
	s.Assert().True(stillThere, "not yet")
	s.Assert().Empty(s.r.Sent)

	s.w.decayFloors(time.Now().Add(rules.CorpseDecay + time.Second))
	_, stillThere = s.w.StartRoom.Inventory.Get(corpse.Id)
	s.Assert().False(stillThere, "gone")
	s.Assert().Equal("the corpse of Target Drone", sent[event.Decayed](s.T(), s.r, 0).Item)

	s.Assert().NotEmpty(s.w.StartRoom.Inventory.FindAll("knife"), "the room's own knife doesn't decay")
}

// Two corpses on the floor: bare "corpse" is the one that just fell, not the
// one about to decay. Found in play -- you kill the second mob, "get all from
// corpse", and loot the first one again.
func (s *lootSuite) TestBareCorpseIsTheNewest() {
	s.killDrone()
	little, exists := s.w.StartRoom.FindMobile("little")
	s.Require().True(exists)
	s.w.combatantDied(little, s.w.StartRoom)
	s.r.Sent = nil

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Look{Target: "corpse", In: true})))

	s.Assert().Equal("the corpse of Little Drone", sent[event.ContainerContents](s.T(), s.r, 0).Container)
}

// After the loot, one more roll: the coins, from power x 3 to twice that for
// the power-3 drone in testcontent's economy -- 9 to 18. The dice: the knife
// misses, the rope (always) drops with no bump, then the coins.
var coinDice = []int{99, 99, 50, 4}

func (s *lootSuite) killForCoins() *object.Instance {
	s.T().Helper()
	s.dice.Load(coinDice)
	corpse := s.killDrone()
	s.r.Sent = nil // the death they watched
	return corpse
}

func (s *lootSuite) TestCoinsGoIntoTheCorpse() {
	corpse := s.killForCoins()

	s.Assert().Equal(9+4, corpse.Coins)
	s.Assert().Equal(map[string]int{"rope": 3}, s.contents(corpse), "the loot rolled first")
}

func (s *lootSuite) TestGetCoinsFromTheCorpse() {
	corpse := s.killForCoins()

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Get{Target: "5 coins", From: "corpse"})))
	s.Assert().Equal(5, s.p.Coins())
	s.Assert().Equal(8, corpse.Coins)
	s.Assert().Equal(event.Got{Actor: "testdood", Item: "5 coins", From: corpse.Definition.ShortDescription}, sent[event.Got](s.T(), s.r, 0))

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Get{Target: "coins", From: "corpse"})))
	s.Assert().Equal(13, s.p.Coins())
	s.Assert().Zero(corpse.Coins)
	s.Assert().Equal(1, corpse.Contents.Len(), "the rope stays: coins were all that was asked for")

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Get{Target: "coins", From: "corpse"})))
	s.Assert().Equal(event.NotInContainer, sent[event.Failed](s.T(), s.r, 2).Code, "none left")
}

// get all takes the coins along with everything else.
func (s *lootSuite) TestGetAllTakesTheCoinsToo() {
	corpse := s.killForCoins()

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Get{Target: "all", From: "corpse"})))
	s.Assert().Equal(13, s.p.Coins())
	s.Assert().Equal("13 coins", sent[event.Got](s.T(), s.r, 0).Item)
	s.Assert().Zero(corpse.Contents.Len())

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Get{Target: "all", From: "corpse"})))
	s.Assert().Equal(event.ContainerEmpty, sent[event.Failed](s.T(), s.r, 2).Code)
}

// A corpse holding nothing but coins isn't empty.
func (s *lootSuite) TestOnlyCoinsIsNotEmpty() {
	corpse := s.killForCoins()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Get{Target: "rope", From: "corpse"})))
	s.Require().Zero(corpse.Contents.Len())

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Get{Target: "all", From: "corpse"})))
	s.Assert().Equal(13, s.p.Coins())
}

func (s *lootSuite) TestLookInShowsTheCoins() {
	s.killForCoins()

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Look{Target: "corpse", In: true})))
	s.Assert().Equal(13, sent[event.ContainerContents](s.T(), s.r, 0).Coins)
}
