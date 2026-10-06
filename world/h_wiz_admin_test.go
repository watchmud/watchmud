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

// testdood is a wizard in Temple Square; ann a player in the store
type wizAdminSuite struct {
	worldTestSuite
	ann    *player.Player
	annRec *player.Recorder
}

func TestWizAdminSuite(t *testing.T) { suite.Run(t, new(wizAdminSuite)) }

func (s *wizAdminSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.p.SetWizard(true)
	s.annRec = &player.Recorder{}
	s.ann = player.NewTestPlayer(uuid.New(), "ann", s.annRec)
	store, found := s.w.findRoomById("wrathrock", "general_store")
	s.Require().True(found)
	s.w.PlacePlayer(s.ann, store)
}

func (s *wizAdminSuite) as(p *player.Player, cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.annRec.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(gameserver.NewHandlerParameter(gameserver.NewTestConn(p), cmd)))
}

func (s *wizAdminSuite) TestGotoAndTransfer() {
	s.as(s.p, command.Goto{Target: "wrathrock/market_square"})
	s.Assert().Equal("market_square", s.w.playerRoom(s.p).Id)
	s.as(s.p, command.Goto{Target: "ANN"})
	s.Assert().Same(s.w.playerRoom(s.ann), s.w.playerRoom(s.p))
	s.as(s.p, command.Goto{Target: "nowhere/at_all"})
	s.Assert().Equal(event.TargetNotFound, sent[event.Failed](s.T(), s.r, 0).Code)

	s.as(s.p, command.Goto{Target: "wrathrock/temple_square"})
	s.as(s.p, command.Transfer{Target: "ann"})
	s.Assert().Same(s.w.StartRoom, s.w.playerRoom(s.ann))
	sent[event.RoomDescription](s.T(), s.annRec, len(s.annRec.Sent)-1)
}

func (s *wizAdminSuite) TestPurge() {
	s.Require().NotEmpty(s.w.StartRoom.Mobiles())
	s.as(s.p, command.Purge{})
	got := sent[event.Purged](s.T(), s.r, 0)
	s.Assert().Positive(got.Mobs)
	s.Assert().Empty(s.w.StartRoom.Mobiles())
	s.Assert().Zero(s.w.StartRoom.Inventory.Len())
	s.Assert().Same(s.w.StartRoom, s.w.playerRoom(s.p), "players are never purged")

	s.as(s.p, command.ZReset{})
	s.Assert().NotEmpty(sent[event.ZoneWasReset](s.T(), s.r, 0).Zone)
	s.Assert().NotEmpty(s.w.StartRoom.Mobiles(), "the reset put them back")
}

func (s *wizAdminSuite) TestEcho() {
	s.as(s.p, command.Echo{Text: "Restarting in five minutes.", Global: true})
	s.Assert().Equal(event.Echoed{Text: "Restarting in five minutes."}, sent[event.Echoed](s.T(), s.annRec, 0))
	s.as(s.p, command.Echo{Text: "The square falls quiet."})
	s.Assert().Empty(s.annRec.Sent, "a room echo stays in the room")
}

func (s *wizAdminSuite) TestUsers() {
	s.as(s.p, command.Users{})
	got := sent[event.UserList](s.T(), s.r, 0)
	s.Require().Len(got.Users, 2)
	s.Assert().True(got.Users[0].Wizard)
}

// muted: no talking to anyone; frozen: look and quit only. Both kept on the
// record, and a second time undoes it.
func (s *wizAdminSuite) TestModeration() {
	s.as(s.p, command.Moderate{Target: "ann"})
	s.Assert().Equal(event.Moderated{Target: "ann", On: true}, sent[event.Moderated](s.T(), s.annRec, 0))
	s.as(s.ann, command.Say{Value: "hi"})
	s.Assert().Equal(event.Muted, sent[event.Failed](s.T(), s.annRec, 0).Code)
	s.as(s.ann, command.Social{Name: "smile"})
	s.Assert().Equal(event.Muted, sent[event.Failed](s.T(), s.annRec, 0).Code)
	s.Assert().True(s.ann.Record().Muted)

	s.as(s.p, command.Moderate{Target: "ann", Freeze: true})
	s.as(s.ann, command.Inventory{})
	s.Assert().Equal(event.Frozen, sent[event.Failed](s.T(), s.annRec, 0).Code)
	s.as(s.ann, command.Look{})
	sent[event.RoomDescription](s.T(), s.annRec, 0)

	s.as(s.p, command.Moderate{Target: "ann", Freeze: true})
	s.as(s.p, command.Moderate{Target: "ann"})
	s.Assert().False(s.ann.Frozen())
	s.Assert().False(s.ann.Muted())

	s.as(s.p, command.Moderate{Target: "testdood"})
	s.Assert().Equal(event.TargetInUse, sent[event.Failed](s.T(), s.r, 0).Code, "not a wizard")
}
