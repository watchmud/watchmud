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

// testdood and ann in Temple Square, bob in the store
type talkSuite struct {
	worldTestSuite
	ann, bob       *player.Player
	annRec, bobRec *player.Recorder
}

func TestTalkSuite(t *testing.T) { suite.Run(t, new(talkSuite)) }

func (s *talkSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.annRec, s.bobRec = &player.Recorder{}, &player.Recorder{}
	s.ann = player.NewTestPlayer(uuid.New(), "ann", s.annRec)
	s.bob = player.NewTestPlayer(uuid.New(), "bob", s.bobRec)
	s.w.PlacePlayer(s.ann, s.w.StartRoom)
	store, found := s.w.findRoomById("wrathrock", "general_store")
	s.Require().True(found)
	s.w.PlacePlayer(s.bob, store)
}

func (s *talkSuite) as(p *player.Player, cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.annRec.Clear()
	s.bobRec.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(gameserver.NewHandlerParameter(gameserver.NewTestConn(p), cmd)))
}

func (s *talkSuite) failed(r *player.Recorder) event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), r, 0).Code
}

func (s *talkSuite) TestSocials() {
	s.as(s.p, command.Social{Name: "smile", Target: "ANN"})
	got := sent[event.Socialized](s.T(), s.annRec, 0)
	s.Assert().Equal("testdood smiles at you.", got.ToTarget)
	s.Assert().Equal("ann", got.Target)

	s.as(s.p, command.Social{Name: "smile", Target: "target"}) // a mob
	s.Assert().Equal("You smile at the Target Drone.", sent[event.Socialized](s.T(), s.r, 0).ToActor)

	s.as(s.p, command.Social{Name: "smile", Target: "bob"})
	s.Assert().Equal(event.TargetNotFound, s.failed(s.r), "bob's in another room")
	s.as(s.p, command.Social{Name: "stretch", Target: "ann"})
	s.Assert().Equal(event.SocialAlone, s.failed(s.r))
	s.as(s.p, command.Social{Name: "florb"})
	s.Assert().Equal(event.UnknownCommand, s.failed(s.r))
	s.Assert().Equal("florb", sent[event.Failed](s.T(), s.r, 0).Verb, "the verb typed, for 'Unknown request: florb'")

	s.as(s.p, command.Socials{})
	s.Assert().Equal([]string{"smile", "stretch"}, sent[event.SocialList](s.T(), s.r, 0).Names)
}

func (s *talkSuite) TestEmote() {
	s.as(s.p, command.Emote{Text: "waves hello."})
	s.Assert().Equal(event.Emoted{Actor: "testdood", Text: "waves hello."}, sent[event.Emoted](s.T(), s.annRec, 0))
	s.Assert().Empty(s.bobRec.Sent)
	s.as(s.p, command.Emote{})
	s.Assert().Equal(event.NoValue, s.failed(s.r))
}

func (s *talkSuite) TestReply() {
	s.as(s.p, command.Reply{Value: "hi"})
	s.Assert().Equal(event.NoOneToReply, s.failed(s.r))

	s.as(s.bob, command.Tell{To: "testdood", Value: "you there?"})
	s.as(s.p, command.Reply{Value: "here"})
	s.Assert().Equal(event.Told{From: "testdood", To: "bob", Value: "here"}, sent[event.Told](s.T(), s.bobRec, 0))
}

func (s *talkSuite) TestWhisperAndAsk() {
	s.as(s.p, command.Whisper{To: "ann", Value: "psst"})
	w := event.Whispered{From: "testdood", To: "ann", Value: "psst"}
	s.Assert().Equal(w, sent[event.Whispered](s.T(), s.annRec, 0))

	s.as(s.p, command.Whisper{To: "bob", Value: "psst"})
	s.Assert().Equal(event.ToPlayerNotFound, s.failed(s.r), "not in the room")

	s.as(s.p, command.Whisper{To: "target", Value: "how much?", Ask: true})
	s.Assert().Equal(event.Whispered{From: "testdood", To: "the Target Drone", Value: "how much?", Ask: true},
		sent[event.Whispered](s.T(), s.annRec, 0), "a mob can be asked; the room sees that much")
}

func (s *talkSuite) TestToggles() {
	s.as(s.ann, command.Toggle{Name: "tells"})
	s.Assert().False(sent[event.Toggled](s.T(), s.annRec, 0).On)
	s.as(s.p, command.Tell{To: "ann", Value: "hi"})
	s.Assert().Equal(event.TellsOff, s.failed(s.r))
	s.as(s.ann, command.Tell{To: "testdood", Value: "hi"})
	s.Assert().Equal(event.YourTellsOff, s.failed(s.annRec))

	s.as(s.ann, command.Toggle{Name: "shout"})
	s.as(s.p, command.TellAll{Value: "hello all"})
	s.Assert().Empty(s.annRec.Sent, "ann's not hearing shouts")
	s.Assert().NotEmpty(s.bobRec.Sent)

	s.as(s.ann, command.Toggle{})
	got := sent[event.Toggles](s.T(), s.annRec, 0)
	s.Assert().False(got.Tells)
	s.Assert().False(got.Shouts)
	s.Assert().True(got.OOC)
	s.Assert().True(s.ann.Record().NoTell, "kept on the record")

	s.as(s.ann, command.Toggle{Name: "volume"})
	s.Assert().Equal(event.UnknownToggle, s.failed(s.annRec))
}
