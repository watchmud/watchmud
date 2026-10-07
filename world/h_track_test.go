package world

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/rules"
)

type trackSuite struct{ worldTestSuite }

func TestTrackSuite(t *testing.T) { suite.Run(t, new(trackSuite)) }

func (s *trackSuite) track(target string) {
	s.T().Helper()
	s.r.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Track{Target: target})))
}

// the heckler is in the store: south to the market, then east -- the first
// step is south
func (s *trackSuite) TestFirstStep() {
	s.track("heckler")
	s.Assert().Equal(event.Tracked{Target: "Heckler", Direction: rules.DirectionSouth}, sent[event.Tracked](s.T(), s.r, 0))
}

func (s *trackSuite) TestHereAndNowhere() {
	s.track("target")
	s.Assert().True(sent[event.Tracked](s.T(), s.r, 0).Here)
	s.track("dragon")
	s.Assert().Equal(event.NoTrail, sent[event.Failed](s.T(), s.r, 0).Code)
	s.track("")
	s.Assert().Equal(event.NoTarget, sent[event.Failed](s.T(), s.r, 0).Code)
}
