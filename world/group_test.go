package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/gameserver"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// testdood leads; ann and bob are in Temple Square with them.
type groupSuite struct {
	worldTestSuite
	ann, bob       *player.Player
	annRec, bobRec *player.Recorder
}

func TestGroupSuite(t *testing.T) {
	suite.Run(t, new(groupSuite))
}

func (s *groupSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.annRec, s.bobRec = &player.Recorder{}, &player.Recorder{}
	s.ann = player.NewTestPlayer(uuid.New(), "ann", s.annRec)
	s.bob = player.NewTestPlayer(uuid.New(), "bob", s.bobRec)
	s.w.PlacePlayer(s.ann, s.w.StartRoom)
	s.w.PlacePlayer(s.bob, s.w.StartRoom)
}

// as runs cmd as p, everyone's recorder cleared first.
func (s *groupSuite) as(p *player.Player, cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.annRec.Clear()
	s.bobRec.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(gameserver.NewHandlerParameter(gameserver.NewTestConn(p), cmd)))
}

func (s *groupSuite) failed(r *player.Recorder) event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), r, 0).Code
}

func (s *groupSuite) TestFollowTellsBothEnds() {
	s.as(s.ann, command.Follow{Target: "TESTDOOD"})

	want := event.Following{Follower: "ann", Leader: "testdood"}
	s.Assert().Equal(want, sent[event.Following](s.T(), s.annRec, 0))
	s.Assert().Equal(want, sent[event.Following](s.T(), s.r, 0))
}

// following a follower follows their leader: one level, no chains
func (s *groupSuite) TestFollowingAFollowerFollowsTheLeader() {
	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.bob, command.Follow{Target: "ann"})

	s.Assert().Equal("testdood", sent[event.Following](s.T(), s.bobRec, 0).Leader)
	s.Assert().Equal([]*player.Player{s.p, s.ann, s.bob}, s.w.groups.members(s.bob))
}

func (s *groupSuite) TestWalkingPullsFollowers() {
	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.bob, command.Follow{Target: "testdood"})

	s.as(s.p, command.Move{Direction: rules.DirectionSouth})

	market := s.w.playerRoom(s.p)
	s.Assert().Same(market, s.w.playerRoom(s.ann))
	s.Assert().Same(market, s.w.playerRoom(s.bob))
	// ann sees testdood go, follows, and is shown where she's come to
	i := indexOf[event.Followed](s.annRec)
	s.Require().GreaterOrEqual(i, 1)
	s.Assert().IsType(event.Left{}, s.annRec.Sent[0])
	s.Assert().Equal(event.Followed{Leader: "testdood", Direction: rules.DirectionSouth}, s.annRec.Sent[i])
	s.Assert().IsType(event.RoomDescription{}, s.annRec.Sent[i+1])
}

// a fight keeps a follower behind, still following
func (s *groupSuite) TestAFightKeepsAFollowerBehind() {
	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.p, command.Assist{Setting: "off"}) // or testdood joins ann's fight and can't walk
	mob, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.Require().NoError(s.w.startFight(s.ann, mob))

	s.as(s.p, command.Move{Direction: rules.DirectionSouth})

	s.Assert().Same(s.w.StartRoom, s.w.playerRoom(s.ann))
	i := indexOf[event.Followed](s.annRec)
	s.Require().GreaterOrEqual(i, 0)
	s.Assert().True(s.annRec.Sent[i].(event.Followed).Fighting)
	s.Assert().Same(s.p, s.w.groups.leaderOf[s.ann])
}

// recall moves one player: nobody is dragged home
func (s *groupSuite) TestOnlyWalkingPulls() {
	s.as(s.ann, command.Follow{Target: "testdood"})
	s.w.movePlayerMagically(s.p, s.w.DeathRoom)
	s.Assert().Same(s.w.StartRoom, s.w.playerRoom(s.ann))
}

func (s *groupSuite) TestStopAndUngroup() {
	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.bob, command.Follow{Target: "testdood"})

	s.as(s.ann, command.Follow{})
	s.Assert().True(sent[event.Following](s.T(), s.r, 0).Stopped)
	s.Assert().Nil(s.w.groups.leaderOf[s.ann])

	s.as(s.p, command.Ungroup{Target: "ann"})
	s.Assert().Equal(event.NotFollowingYou, s.failed(s.r))

	s.as(s.p, command.Ungroup{Target: "bob"})
	s.Assert().True(sent[event.Following](s.T(), s.bobRec, 0).Stopped)
	s.Assert().Nil(s.w.groups.members(s.p))

	s.as(s.p, command.Ungroup{})
	s.Assert().Equal(event.NoFollowers, s.failed(s.r))
	s.as(s.ann, command.Follow{})
	s.Assert().Equal(event.NotFollowing, s.failed(s.annRec))
}

func (s *groupSuite) TestFollowRefusals() {
	s.as(s.ann, command.Follow{Target: "nobody"})
	s.Assert().Equal(event.ToPlayerNotFound, s.failed(s.annRec))

	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.ann, command.Follow{Target: "testdood"})
	s.Assert().Equal(event.AlreadyFollowing, s.failed(s.annRec))
	s.as(s.p, command.Follow{Target: "ann"})
	s.Assert().Equal(event.FollowsYou, s.failed(s.r))
}

// a leader who goes off to follow someone else takes nobody along
func (s *groupSuite) TestALeaderWhoFollowsDisbands() {
	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.p, command.Follow{Target: "bob"})

	s.Assert().Nil(s.w.groups.leaderOf[s.ann])
	s.Assert().Same(s.bob, s.w.groups.leaderOf[s.p])
	s.Assert().True(sent[event.Following](s.T(), s.annRec, 0).Stopped)
}

func (s *groupSuite) TestGroupAndGtell() {
	s.as(s.p, command.Group{})
	s.Assert().Equal(event.NotInGroup, s.failed(s.r))
	s.as(s.p, command.GroupTell{Value: "hi"})
	s.Assert().Equal(event.NotInGroup, s.failed(s.r))

	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.ann, command.Group{})
	list := sent[event.GroupList](s.T(), s.annRec, 0)
	s.Require().Len(list.Members, 2)
	s.Assert().Equal("testdood", list.Members[0].Name)
	s.Assert().True(list.Members[0].Leader)
	s.Assert().Equal(s.w.StartRoom.Name, list.Members[1].Room)

	s.as(s.ann, command.GroupTell{Value: "ready?"})
	told := event.GroupTold{Speaker: "ann", Value: "ready?"}
	s.Assert().Equal(told, sent[event.GroupTold](s.T(), s.r, 0))
	s.Assert().Equal(told, sent[event.GroupTold](s.T(), s.annRec, 0))
	s.Assert().Empty(s.bobRec.Sent, "bob isn't in it")
}

// logging out leaves the group from either end
func (s *groupSuite) TestLeavingTheWorld() {
	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.bob, command.Follow{Target: "testdood"})

	s.w.RemovePlayer(s.p)

	s.Assert().Nil(s.w.groups.leaderOf[s.ann])
	s.Assert().Nil(s.w.groups.leaderOf[s.bob])
	s.Assert().True(sent[event.Following](s.T(), s.annRec, 0).Stopped)
	s.Assert().Empty(s.w.groups.followers)
}

// indexOf is where the first T is in what r received, or -1.
func indexOf[T any](r *player.Recorder) int {
	for i, m := range r.Sent {
		if _, ok := m.(T); ok {
			return i
		}
	}
	return -1
}

// the group joins a fight any of them is drawn into, in the room
func (s *groupSuite) TestAssist() {
	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.bob, command.Follow{Target: "testdood"})
	s.as(s.bob, command.Assist{Setting: "off"})
	s.Assert().False(sent[event.AssistSet](s.T(), s.bobRec, 0).On)

	s.as(s.p, command.Kill{Target: "target"})

	mob, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.Assert().True(s.w.fightLedger.InFight(s.ann))
	s.Assert().Equal(mob, s.w.fightLedger.GetFight(s.ann).Fightee, "ann fights what testdood fights")
	s.Assert().False(s.w.fightLedger.InFight(s.bob), "bob turned assist off")
	i := indexOf[event.Assisted](s.annRec)
	s.Require().GreaterOrEqual(i, 0)
	s.Assert().Equal(event.Assisted{Actor: "ann", Member: "testdood", Target: mob.Name()}, s.annRec.Sent[i])
	s.Assert().Same(s.p, s.w.fightLedger.GetFight(mob).Fightee.(*player.Player), "the mob stays on whoever engaged first")
}

// a follower set upon brings the leader in too; someone elsewhere stays out
func (s *groupSuite) TestAssistGoesBothWays() {
	s.as(s.ann, command.Follow{Target: "testdood"})
	s.as(s.bob, command.Follow{Target: "testdood"})
	store, found := s.w.findRoomById("wrathrock", "general_store")
	s.Require().True(found)
	s.w.movePlayerMagically(s.bob, store)
	mob, found := s.w.StartRoom.FindMobile("target")
	s.Require().True(found)

	s.Require().NoError(s.w.startFight(mob, s.ann)) // aggro on ann

	s.Assert().True(s.w.fightLedger.InFight(s.p))
	s.Assert().False(s.w.fightLedger.InFight(s.bob))
}

func (s *groupSuite) TestAssistSetting() {
	s.as(s.ann, command.Assist{})
	s.Assert().False(sent[event.AssistSet](s.T(), s.annRec, 0).On, "on by default, so a bare assist turns it off")
	s.as(s.ann, command.Assist{})
	s.Assert().True(sent[event.AssistSet](s.T(), s.annRec, 0).On)
	s.as(s.ann, command.Assist{Setting: "sideways"})
	s.Assert().Equal(event.BadRequest, s.failed(s.annRec))
}
