package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// Where a player is standing goes into their record, and comes back out of it
// when they return.
type locationSuite struct {
	worldTestSuite
}

func TestLocationSuite(t *testing.T) {
	suite.Run(t, new(locationSuite))
}

func (s *locationSuite) handle(cmd command.Command) {
	s.T().Helper()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(cmd)))
}

func (s *locationSuite) saved() *player.Record {
	s.T().Helper()
	rec, found, err := s.w.store.Load(s.p.Name())
	s.Require().NoError(err)
	s.Require().True(found)
	return rec
}

// Logging out takes the player out of the room before saving; the room they
// were in still has to make it into the record.
func (s *locationSuite) TestLogoutRecordsTheRoom() {
	s.handle(command.Move{Direction: rules.DirectionSouth})
	s.handle(command.Logout{})

	rec := s.saved()
	s.Assert().Equal("market_square", rec.LastRoomId)
}

func (s *locationSuite) returning() *player.Player {
	p := player.NewTestPlayer(uuid.New(), "returner", nil)
	return p
}

func (s *locationSuite) TestReturnPlayerToTheirRoom() {
	p := s.returning()

	s.w.ReturnPlayer(p, "wrathrock", "market_square")

	market, _ := s.w.findRoomById("wrathrock", "market_square")
	s.Assert().Equal(market, s.w.playerRoom(p))
	s.Assert().Contains(market.Players(), p)
}

// A room a content edit took away sends them to the start room rather than
// leaving them nowhere.
func (s *locationSuite) TestReturnPlayerToARoomThatIsGone() {
	p := s.returning()

	s.w.ReturnPlayer(p, "wrathrock", "demolished")

	s.Assert().Equal(s.w.StartRoom, s.w.playerRoom(p))
}

// A record from before location was saved says nothing at all.
func (s *locationSuite) TestReturnPlayerWithNoRoom() {
	p := s.returning()

	s.w.ReturnPlayer(p, "", "")

	s.Assert().Equal(s.w.StartRoom, s.w.playerRoom(p))
}

// Arriving: the room hears about it, and the player sees the room -- but not
// the announcement of their own arrival.
func (s *locationSuite) TestArrive() {
	rec := &player.Recorder{}
	q := player.NewTestPlayer(uuid.New(), "newcomer", rec)
	s.w.PlacePlayer(q, s.w.StartRoom)

	s.w.Arrive(q)

	s.Require().Len(s.r.Sent, 1)
	s.Assert().Equal("newcomer", sent[event.EnteredGame](s.T(), s.r, 0).Actor)

	s.Require().Len(rec.Sent, 3)
	s.Assert().Equal(event.Color{On: true}, sent[event.Color](s.T(), rec, 0), "the setting, ahead of the room")
	s.Assert().Equal(s.w.StartRoom.Name, sent[event.RoomDescription](s.T(), rec, 1).Name)
	// a test player predates the temple token, so is handed one -- after the room
	s.Assert().IsType(event.Received{}, rec.Sent[2])
}
