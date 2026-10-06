package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/testdice"
)

type miscSuite struct {
	worldTestSuite
	ann    *player.Player
	annRec *player.Recorder
}

func TestMiscSuite(t *testing.T) { suite.Run(t, new(miscSuite)) }

func (s *miscSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.annRec = &player.Recorder{}
	s.ann = player.NewTestPlayer(uuid.New(), "ann", s.annRec)
	s.w.PlacePlayer(s.ann, s.w.StartRoom)
}

func (s *miscSuite) as(p *player.Player, cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.annRec.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(gameserver.NewHandlerParameter(gameserver.NewTestConn(p), cmd)))
}

func (s *miscSuite) TestSplit() {
	s.p.AddCoins(31)
	s.as(s.p, command.Split{Amount: "30"})
	s.Assert().Equal(event.NoOneToSplit, sent[event.Failed](s.T(), s.r, 0).Code, "not in a group")

	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.p, command.Split{Amount: "31"})
	s.Assert().Equal(event.SplitCoins{Actor: "testdood", Each: 15, Among: 2}, sent[event.SplitCoins](s.T(), s.annRec, 0))
	s.Assert().Equal(15, s.ann.Coins())
	s.Assert().Equal(16, s.p.Coins(), "the odd coin stays with the splitter")

	s.as(s.p, command.Split{Amount: "100"})
	s.Assert().Equal(event.NotEnoughCoins, sent[event.Failed](s.T(), s.r, 0).Code)
}

func (s *miscSuite) TestWhere() {
	s.as(s.p, command.Where{})
	got := sent[event.WhereList](s.T(), s.r, 0)
	s.Assert().Len(got.Players, 2)
}

// Wrathrock keeps Seattle's time, daylight saving and all
func (s *miscSuite) TestTime() {
	s.w.now = func() time.Time { return time.Date(2026, 10, 7, 4, 41, 0, 0, time.UTC) }
	s.as(s.p, command.Time{})
	s.Assert().Equal(event.TimeOfDay{Clock: "9:41 pm", Day: "Tuesday, October 6", Part: "night"}, sent[event.TimeOfDay](s.T(), s.r, 0))
}

// wimpy: hit below the line in a fight, the player runs on their own
func (s *miscSuite) TestWimpy() {
	s.as(s.p, command.Wimpy{Amount: "50"})
	s.Assert().Equal(event.WimpySet{At: 50, Changed: true}, sent[event.WimpySet](s.T(), s.r, 0))
	s.Assert().Equal(50, s.p.Record().Wimpy, "kept on the record")
	s.as(s.p, command.Wimpy{Amount: "500"})
	s.Assert().Equal(event.BadRequest, sent[event.Failed](s.T(), s.r, 0).Code)

	mob, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.Require().NoError(s.w.startFight(mob, s.p))
	s.p.TakeMeleeDamage(s.p.CurrentHealth() - 52)
	dice := testdice.New()
	// the mob's swing: a 20 and 5 damage, to 47; then the first way out works
	dice.Load([]int{20, 5, 0, 0, 0, 0, 0, 0})
	s.w.roller = dice
	s.r.Clear()
	s.w.DoViolence(1000)

	s.Assert().GreaterOrEqual(indexOf[event.Panicked](s.r), 0, "it panicked")
}
