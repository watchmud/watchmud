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

func (s *handleColorSuite) screenReader(setting string) {
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.ScreenReader{Setting: setting})))
}

// Screen-reader mode is color's twin: off unless asked, kept on the record.
func (s *handleColorSuite) TestScreenReader() {
	s.Require().False(s.p.ScreenReader())

	s.screenReader("")
	s.Assert().True(s.p.ScreenReader(), "a bare screenreader switches it")
	s.Assert().Equal(event.ScreenReader{On: true, Changed: true}, sent[event.ScreenReader](s.T(), s.r, 0))
	s.Assert().True(s.w.record(s.p).ScreenReader)

	s.screenReader("off")
	s.Assert().False(s.p.ScreenReader())

	s.screenReader("loud")
	s.Assert().Equal(event.Failed{Verb: "screenreader", Code: event.BadRequest}, sent[event.Failed](s.T(), s.r, 2))

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Toggle{Name: "screenreader"})))
	s.Assert().True(s.p.ScreenReader(), "toggle switches it too")
}
