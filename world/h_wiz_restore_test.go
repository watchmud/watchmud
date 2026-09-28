package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
)

// restore heals its target to full. On a player it used to tell the room they
// were restored and do nothing -- the call was commented out, and nothing
// tested it. Driven through the handler rather than HandleIncomingMessage,
// since the wizard gate has its own suite.
type restoreSuite struct {
	worldTestSuite
}

func TestRestoreSuite(t *testing.T) {
	suite.Run(t, new(restoreSuite))
}

func (s *restoreSuite) restore(target string) {
	cmd := command.Restore{Target: target}
	s.w.handleRestore(s.handlerParameter(cmd), cmd)
}

func (s *restoreSuite) TestRestoresAPlayer() {
	s.p.TakeMeleeDamage(40)
	s.Require().Less(s.p.CurrentHealth(), s.p.MaxHealth())

	s.restore("testdood")

	s.Assert().Equal(s.p.MaxHealth(), s.p.CurrentHealth())
	s.Assert().Equal(event.Restored{Target: "testdood", IsPlayer: true}, sent[event.Restored](s.T(), s.r, 0))
}

func (s *restoreSuite) TestRestoresAMob() {
	drone, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	drone.TakeMeleeDamage(5)
	s.Require().Less(drone.CurHealth, drone.Definition.MaxHealth)

	s.restore("target")

	s.Assert().Equal(drone.Definition.MaxHealth, drone.CurHealth)
	s.Assert().Equal(event.Restored{Target: drone.Name(), IsPlayer: false}, sent[event.Restored](s.T(), s.r, 0))
}

func (s *restoreSuite) TestNobodyThere() {
	s.restore("nobody")

	s.Assert().Equal(event.TargetNotFound, sent[event.Failed](s.T(), s.r, 0).Code)
}
