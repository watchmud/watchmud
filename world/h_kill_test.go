package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
)

type handleKillSuite struct {
	worldTestSuite
}

func TestHandleKillSuite(t *testing.T) {
	suite.Run(t, new(handleKillSuite))
}

func (s *handleKillSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
}

func (s *handleKillSuite) kill(target string) {
	s.T().Helper()
	cmd := command.Kill{Target: target}
	s.w.handleKill(gameserver.NewHandlerParameter(s.c, cmd), cmd)
}

func (s *handleKillSuite) TestSuccess() {
	mob, exists := s.w.StartRoom.FindMobile("target")
	s.Assert().True(exists)

	s.kill("target")

	s.Assert().Equal(1, len(s.r.Sent))
	attacking := s.r.Sent[0].(event.Attacking)
	s.Assert().Equal("Target Drone", attacking.Target)

	s.Assert().True(s.w.fightLedger.IsFighting(s.p))
	s.Assert().Equal(mob, s.w.fightLedger.GetFight(s.p).Fightee)
	s.Assert().Equal(s.p, s.w.fightLedger.GetFight(s.p).Fighter)
	// clear the players sent queue
	s.r.Clear()
}

func (s *handleKillSuite) TestAlreadyFighting() {
	s.TestSuccess()

	// now let's attack something else
	s.kill("other")

	s.Assert().Equal(1, len(s.r.Sent))
	result := s.r.Sent[0].(event.Failed)
	s.Assert().Equal(event.AlreadyFighting, result.Code)
}

func (s *handleKillSuite) TestNoTarget() {
	s.kill("not-here")

	result := s.r.Sent[0].(event.Failed)
	s.Assert().Equal(event.TargetNotFound, result.Code)
}

func (s *handleKillSuite) TestNoFight() {
	mob, _ := s.w.StartRoom.FindMobile("target")
	mob.Definition.SetFlag(rules.MobileFlagPlayerCantFight)

	s.kill("target")

	result := s.r.Sent[0].(event.Failed)
	s.Assert().Equal(event.NoFight, result.Code)
}

func (s *handleKillSuite) TestNoFightInRoom() {
	s.w.StartRoom.SetFlag(spaces.RoomFlagNoFight)

	s.kill("target")

	result := s.r.Sent[0].(event.Failed)
	s.Assert().Equal(event.NoFightRoom, result.Code)
}
