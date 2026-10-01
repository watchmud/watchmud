package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
)

type handleColorSuite struct {
	worldTestSuite
}

func TestHandleColorSuite(t *testing.T) {
	suite.Run(t, new(handleColorSuite))
}

func (s *handleColorSuite) color(setting string) {
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Color{Setting: setting})))
}

func (s *handleColorSuite) TestColor() {
	s.Require().True(s.p.Color(), "on unless they say otherwise")

	s.color("off")
	s.Assert().False(s.p.Color())
	s.Assert().Equal(event.Color{On: false, Changed: true}, sent[event.Color](s.T(), s.r, 0))

	s.color("")
	s.Assert().True(s.p.Color(), "a bare color switches it")
	s.Assert().Equal(event.Color{On: true, Changed: true}, sent[event.Color](s.T(), s.r, 1))

	s.color("on")
	s.Assert().True(s.p.Color(), "on is on, not a switch")

	s.color("purple")
	s.Assert().True(s.p.Color())
	s.Assert().Equal(event.Failed{Verb: "color", Code: event.BadRequest}, sent[event.Failed](s.T(), s.r, 3))
}

// Off has to survive a logout, or it's a setting the player makes every time.
func (s *handleColorSuite) TestColor_onTheRecord() {
	s.color("off")

	s.Assert().True(s.w.record(s.p).NoColor)
}
