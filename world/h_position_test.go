package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

type positionSuite struct{ worldTestSuite }

func TestPositionSuite(t *testing.T) { suite.Run(t, new(positionSuite)) }

func (s *positionSuite) do(cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(cmd)))
}

func (s *positionSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

func (s *positionSuite) TestSitRestSleepWake() {
	s.do(command.Position{To: "rest"})
	s.Assert().Equal(event.PositionChanged{Actor: "testdood", To: "resting"}, sent[event.PositionChanged](s.T(), s.r, 0))
	s.do(command.Position{To: "rest"})
	s.Assert().Equal(event.AlreadyPosition, s.failed())

	s.do(command.Move{Direction: rules.DirectionSouth})
	s.Assert().Equal(event.NotStanding, s.failed(), "no walking off your feet")

	s.do(command.Position{To: "sleep"})
	s.do(command.Look{})
	s.Assert().Equal(event.Asleep, s.failed())
	s.do(command.Inventory{})
	sent[event.Inventory](s.T(), s.r, 0) // a sleeper can still check their pack

	s.do(command.Position{Wake: true})
	s.Assert().Equal(event.PositionChanged{Actor: "testdood", Woke: true}, sent[event.PositionChanged](s.T(), s.r, 0))
	s.Assert().Equal(player.Standing, s.p.Position())
	s.do(command.Position{Wake: true})
	s.Assert().Equal(event.AlreadyPosition, s.failed())
}

// a fight brings you to your feet, and you can't sit down in one
func (s *positionSuite) TestFights() {
	s.do(command.Position{To: "sleep"})
	mob, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.Require().NoError(s.w.startFight(mob, s.p))
	s.Assert().Equal(player.Standing, s.p.Position())

	s.do(command.Position{To: "sit"})
	s.Assert().Equal(event.InAFight, s.failed())
}

// resting and sleeping heal faster than standing about
func (s *positionSuite) TestFasterRegen() {
	healed := func(pos player.Position) int {
		s.p.SetPosition(pos)
		s.p.TakeMeleeDamage(60)
		before := s.p.CurrentHealth()
		s.w.Regenerate()
		after := s.p.CurrentHealth()
		s.p.RestoreMaxHealth()
		return after - before
	}
	stand, rest, sleep := healed(player.Standing), healed(player.Resting), healed(player.Sleeping)
	s.Assert().Greater(rest, stand)
	s.Assert().Greater(sleep, rest)
}

// the room says how you are
func (s *positionSuite) TestTheRoomSees() {
	s.p.SetPosition(player.Resting)
	desc := s.w.StartRoom.DescriptionExcept(nil)
	for i, name := range desc.Players {
		if name == "testdood" {
			s.Assert().Equal("resting", desc.PlayerPositions[i])
		}
	}
}
