package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/testdice"
)

type slaySuite struct {
	worldTestSuite
}

func TestSlaySuite(t *testing.T) {
	suite.Run(t, new(slaySuite))
}

func (s *slaySuite) slay(target string) {
	s.T().Helper()
	s.r.Clear()
	cmd := command.Slay{Target: target}
	s.w.handleSlay(s.handlerParameter(cmd), cmd)
}

// the whole of a real death: the room is told, and the corpse holds the loot
// (the target drone drops a rope on any roll; see lootSuite for the table)
func (s *slaySuite) TestSlay() {
	s.p.SetWizard(true)
	dice := testdice.New()
	dice.Load([]int{99, 0, 99, 0}) // knife misses, rope hits with no bump, coins
	s.w.roller = dice
	_, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)

	s.slay("target")

	_, found = s.w.StartRoom.FindMobile("target")
	s.Assert().False(found, "gone from the room")
	s.Assert().Equal(event.Slain{Actor: "testdood", Target: "Target Drone"}, sent[event.Slain](s.T(), s.r, 0))
	s.Assert().Equal(event.Died{Target: "Target Drone"}, sent[event.Died](s.T(), s.r, 1))

	corpse := s.w.StartRoom.Inventory.FindAll("corpse")
	s.Require().Len(corpse, 1)
	s.Assert().NotEmpty(corpse[0].Contents.FindAll("rope"), "the loot was rolled")
}

// a fight with it ends, as it would with any other death
func (s *slaySuite) TestEndsTheFight() {
	s.p.SetWizard(true)
	drone, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.Require().NoError(s.w.fightLedger.Fight(s.p, drone))

	s.slay("target")

	s.Assert().False(s.w.fightLedger.InFight(s.p))
}

// mobs only: a player's name finds nothing to slay
func (s *slaySuite) TestNotPlayers() {
	s.p.SetWizard(true)

	s.slay("testdood")

	s.Assert().Equal(event.TargetNotFound, sent[event.Failed](s.T(), s.r, 0).Code)
	s.Assert().Equal(100, s.p.CurrentHealth())
}

func (s *slaySuite) TestNoTarget() {
	s.p.SetWizard(true)

	s.slay("")

	s.Assert().Equal(event.NoTarget, sent[event.Failed](s.T(), s.r, 0).Code)
}
