package world

import (
	"os"
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/memstore"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/script"
	"github.com/watchmud/watchmud/spaces"
	"github.com/watchmud/watchmud/testdice"
)

// The real Barrow-King in his real throne room, with a player in front of him.
type kingSuite struct {
	suite.Suite
	w     *World
	dice  *testdice.LoadedDice
	room  *spaces.Room
	king  *mobile.Instance
	p     *player.Player
	heard *player.Recorder
}

func TestKingSuite(t *testing.T) {
	suite.Run(t, new(kingSuite))
}

func (s *kingSuite) SetupTest() {
	content, err := loader.LoadContent(os.DirFS("../content"))
	s.Require().NoError(err)
	s.dice = testdice.New()
	s.w, err = New(content, memstore.New(), s.dice)
	s.Require().NoError(err)
	s.room = s.w.Zone("barrow").Rooms["throne_room"]
	var found bool
	s.king, found = s.room.FindMobile("king")
	s.Require().True(found)
	s.heard = &player.Recorder{}
	s.p = player.NewTestPlayer(uuid.New(), "testdood", s.heard)
	s.w.PlacePlayer(s.p, s.room)
}

func (s *kingSuite) foe() script.Foe { return script.Foe{Name: "testdood", IsPlayer: true} }

func (s *kingSuite) skeletons() int {
	n := 0
	for _, m := range s.room.Mobiles() {
		if m.Definition.Id == "barrow_skeleton" && m.Summoner == s.king {
			n++
		}
	}
	return n
}

func (s *kingSuite) said() []string {
	var lines []string
	for _, m := range s.heard.Sent {
		if e, ok := m.(event.Said); ok {
			lines = append(lines, e.Value)
		}
	}
	return lines
}

// above half, nothing: the taunt roll fails (99 is not under 15)
func (s *kingSuite) TestAboveHalfNoGuard() {
	s.king.CurHealth = s.king.Definition.MaxHealth/2 + 1
	s.dice.Load([]int{99})

	s.w.scripts.FightPulse(s.king, s.foe())

	s.Assert().Equal(0, s.skeletons())
}

// at half: his line, two skeletons, no taunt that round; and never twice
func (s *kingSuite) TestAtHalfOnce() {
	s.king.CurHealth = s.king.Definition.MaxHealth / 2
	s.dice.Load([]int{0, 0}) // each skeleton picks the one player

	s.w.scripts.FightPulse(s.king, s.foe())

	s.Assert().Equal(2, s.skeletons())
	s.Assert().Equal([]string{"Rise, my guard! Rise and defend your king!"}, s.said())

	s.dice.Load([]int{99}) // the next round's taunt roll, and no more summons
	s.w.scripts.FightPulse(s.king, s.foe())
	s.Assert().Equal(2, s.skeletons())
}

// Review focus 3: a fresh fight -- the old guard gone to dust -- gets a fresh summon
func (s *kingSuite) TestFreshFightSummonsAgain() {
	s.king.CurHealth = s.king.Definition.MaxHealth / 2
	s.dice.Load([]int{0, 0})
	s.w.scripts.FightPulse(s.king, s.foe())
	s.w.crumbleSummonsOf(s.king)

	s.dice.Load([]int{0, 0, 0}) // the opener's pick, then two targets
	s.w.scripts.FightStart(s.king, s.foe())
	s.w.scripts.FightPulse(s.king, s.foe())

	s.Assert().Equal(2, s.skeletons())
}
