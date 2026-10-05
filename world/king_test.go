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

func (s *kingSuite) skeletons() int {
	n := 0
	for _, m := range s.w.Mobiles() {
		if m.Summoner == s.king {
			n++
		}
	}
	return n
}

// fight starts one between the player and the King; his opener picks a line
func (s *kingSuite) fight() {
	s.dice.Load([]int{0})
	s.Require().NoError(s.w.startFight(s.p, s.king))
}

func (s *kingSuite) half() {
	s.king.CurHealth = s.king.Definition.MaxHealth / 2
	s.w.scripts.FightPulse(s.king, s.foe())
}

// at half: his line, a pulse of nothing, then his guard; and never twice
func (s *kingSuite) TestAtHalfPauses() {
	s.fight()
	s.half()
	said := s.said()
	s.Assert().Equal("Rise, my guard! Rise and defend your king!", said[len(said)-1])
	s.Assert().Equal(0, s.skeletons())

	s.w.ResumeScripts()
	s.Assert().Equal(0, s.skeletons(), "a pulse in: still the pause")

	s.dice.Load([]int{0, 0})
	s.w.ResumeScripts()
	s.Assert().Equal(2, s.skeletons())

	s.dice.Load([]int{99})
	s.w.scripts.FightPulse(s.king, s.foe())
	s.Assert().Equal(2, s.skeletons(), "once a fight")
}

// Review focus 2: everyone gone before the guard rises
func (s *kingSuite) TestFleeDuringThePause() {
	s.fight()
	s.half()
	s.w.fightLedger.EndAllFightsWith(s.p.Id())

	s.w.ResumeScripts()
	s.w.ResumeScripts()

	s.Assert().Equal(0, s.skeletons())
}

// Review focus 1: slain mid-pause -- no guard, not even in the void
func (s *kingSuite) TestSlainDuringThePause() {
	s.fight()
	s.half()
	s.dice.Load([]int{99, 99, 99, 0}) // his three loot rolls miss; coins
	s.w.combatantDied(s.king, s.room)

	s.w.ResumeScripts()
	s.w.ResumeScripts()

	s.Assert().Equal(0, s.skeletons())
}

// Review focus 2: a fresh fight, after the old guard is dust, gets a fresh guard
func (s *kingSuite) TestFreshFightSummonsAgain() {
	s.fight()
	s.half()
	s.w.ResumeScripts()
	s.dice.Load([]int{0, 0})
	s.w.ResumeScripts()
	s.w.fightLedger.EndAllFightsWith(s.p.Id())
	s.w.crumbleSummonsOf(s.king)

	s.fight()
	s.half()
	s.w.ResumeScripts()
	s.dice.Load([]int{0, 0})
	s.w.ResumeScripts()

	s.Assert().Equal(2, s.skeletons())
}
