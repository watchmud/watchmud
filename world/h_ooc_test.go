package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
)

// testdood in Temple Square; ann in the store, off in another room
type oocSuite struct {
	worldTestSuite
	ann    *player.Player
	annRec *player.Recorder
}

func TestOOCSuite(t *testing.T) { suite.Run(t, new(oocSuite)) }

func (s *oocSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.annRec = &player.Recorder{}
	s.ann = player.NewTestPlayer(uuid.New(), "ann", s.annRec)
	store, found := s.w.findRoomById("wrathrock", "general_store")
	s.Require().True(found)
	s.w.PlacePlayer(s.ann, store)
}

func (s *oocSuite) ooc(p *player.Player, value string) {
	s.T().Helper()
	s.r.Clear()
	s.annRec.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(gameserver.NewHandlerParameter(gameserver.NewTestConn(p), command.OOC{Value: value})))
}

// everyone on it hears it, wherever they are, the speaker too
func (s *oocSuite) TestEveryoneOnItHears() {
	s.ooc(s.p, "anyone for the mill?")
	want := event.OOCSaid{Speaker: "testdood", Value: "anyone for the mill?"}
	s.Assert().Equal(want, sent[event.OOCSaid](s.T(), s.r, 0))
	s.Assert().Equal(want, sent[event.OOCSaid](s.T(), s.annRec, 0))
}

func (s *oocSuite) TestLeavingAndComingBack() {
	s.ooc(s.ann, "OFF")
	s.Assert().False(sent[event.OOCSet](s.T(), s.annRec, 0).On)
	s.Assert().False(s.ann.OOC())

	s.ooc(s.p, "hello?")
	s.Assert().Empty(s.annRec.Sent, "off the channel, ann hears nothing")

	s.ooc(s.ann, "hello")
	s.Assert().Equal(event.OffChannel, sent[event.Failed](s.T(), s.annRec, 0).Code, "nor speaks on it")

	s.ooc(s.ann, "on")
	s.Assert().True(s.ann.OOC())
	s.Assert().True(s.ann.Record().NoOOC == false)
}

func (s *oocSuite) TestNothingToSay() {
	s.ooc(s.p, "")
	s.Assert().Equal(event.NoValue, sent[event.Failed](s.T(), s.r, 0).Code)
}
