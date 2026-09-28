package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/event"
)

type welcomeSuite struct {
	worldTestSuite
}

func TestWelcomeSuite(t *testing.T) {
	suite.Run(t, new(welcomeSuite))
}

// The text is content: whatever settings.json says, said as-is.
func (s *welcomeSuite) TestWelcomeSaysTheSetting() {
	s.w.Welcome(s.p)

	s.Assert().Equal(event.Welcome{Text: "Welcome! The fighting is south."}, sent[event.Welcome](s.T(), s.r, 0))
}

// No setting, no line -- not an empty one.
func (s *welcomeSuite) TestNoWelcomeSaysNothing() {
	s.w.content.Settings.Welcome = ""

	s.w.Welcome(s.p)

	s.Assert().Empty(s.r.Sent)
}
