package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/spaces"
	"github.com/watchmud/watchmud/testdice"
)

// The heckler (testcontent/world/wrathrock/scripts/heckler.lua) in the
// general store, with the player and a second pair of ears beside him.
type scriptsSuite struct {
	worldTestSuite
	store   *spaces.Room
	heckler *mobile.Instance
	ears    *player.Recorder
	dice    *testdice.LoadedDice
}

func TestScriptsSuite(t *testing.T) {
	suite.Run(t, new(scriptsSuite))
}

func (s *scriptsSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.store = s.w.Zone("wrathrock").Rooms["general_store"]
	heckler, found := s.store.FindMobile("heckler")
	s.Require().True(found)
	s.heckler = heckler

	s.w.movePlayer(s.p, rules.DirectionNone, s.store)
	s.ears = &player.Recorder{}
	s.w.PlacePlayer(player.NewTestPlayer(uuid.New(), "listener", s.ears), s.store)

	s.dice = testdice.New()
	s.w.roller = s.dice
	s.r.Clear()
}

// said is every line the heckler's room heard, in order.
func (s *scriptsSuite) said() []string {
	var lines []string
	for _, m := range s.ears.Sent {
		if e, ok := m.(event.Said); ok {
			lines = append(lines, e.Speaker+": "+e.Value)
		}
	}
	return lines
}

func (s *scriptsSuite) kill() {
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Kill{Target: "heckler"})))
}

// kill: "Ok." to the killer first, then the opener, which the room hears.
func (s *scriptsSuite) TestKillFiresTheOpener() {
	s.kill()

	s.Assert().IsType(event.Attacking{}, s.r.Sent[0])
	s.Assert().Equal(event.Said{Speaker: "Heckler", Value: "Come on then, testdood!"}, sent[event.Said](s.T(), s.r, 1))
	s.Assert().Equal([]string{"Heckler: Come on then, testdood!"}, s.said())
}

// Aggro starts a fight too, and gets the same opener.
func (s *scriptsSuite) TestAggroFiresTheOpener() {
	s.heckler.Definition.SetFlag(mobile.Aggressive)

	s.w.DoMobileActivity()

	s.Require().True(s.w.fightLedger.IsFighting(s.heckler))
	s.Assert().Equal([]string{"Heckler: Come on then, testdood!"}, s.said())
}

// Memory is the heckler's own, and lasts as long as he does: a second fight
// with him is "You again", a fresh heckler after a reset starts over.
func (s *scriptsSuite) TestMemoryLastsAsLongAsTheMob() {
	s.kill()
	s.w.fightLedger.EndAllFightsWith(s.p.Id())
	s.kill()

	s.w.combatantDied(s.heckler, s.store)
	s.Require().Empty(s.w.Zone("wrathrock").Reset(s.w.occupancy))
	fresh, found := s.store.FindMobile("heckler")
	s.Require().True(found)
	s.Require().NotSame(s.heckler, fresh)
	s.w.fightLedger.EndAllFightsWith(s.p.Id())
	s.kill()

	s.Assert().Equal([]string{
		"Heckler: Come on then, testdood!",
		"Heckler: You again, testdood?",
		"Heckler: Come on then, testdood!",
	}, s.said())
}

// Joining a fight the heckler is already in, and his retargeting when his
// first opponent leaves, are not starts: one opener, not three.
func (s *scriptsSuite) TestNoOpenerWhenAlreadyFighting() {
	otherRec := &player.Recorder{}
	other := player.NewTestPlayer(uuid.New(), "other", otherRec)
	s.w.PlacePlayer(other, s.store)

	s.kill()
	s.Require().NoError(s.w.startFight(other, s.heckler))
	s.w.fightLedger.EndAllFightsWith(s.p.Id()) // he turns on other

	s.Require().Same(other, s.w.fightLedger.GetFight(s.heckler).Fightee)
	s.Assert().Equal([]string{"Heckler: Come on then, testdood!"}, s.said())
}

// oneSwing leaves exactly one fight -- the heckler at the player -- so
// loaded dice can be aimed at it.
func (s *scriptsSuite) oneSwing() {
	s.Require().NoError(s.w.startFight(s.heckler, s.p))
	s.w.fightLedger.EndFight(s.p)
	s.ears.Clear()
	s.r.Clear()
}

// A round: the blow, then the taunt.
func (s *scriptsSuite) TestPulseTauntsAfterTheBlow() {
	s.oneSwing()
	s.dice.Load([]int{1, 0, 0}) // d20 1: a miss; chance(50) 0: yes; pick 0: "Is that all?"

	s.w.DoViolence(5)

	s.Assert().IsType(event.Struck{}, s.ears.Sent[0])
	s.Assert().Equal(event.Said{Speaker: "Heckler", Value: "Is that all?"}, s.ears.Sent[1])
}

// A killing blow gets no taunt. Spare dice are loaded, so a pulse fired by
// mistake would say something rather than fail on empty dice.
func (s *scriptsSuite) TestNoTauntOverTheBody() {
	s.oneSwing()
	s.p.TakeMeleeDamage(s.p.CurrentHealth() - 1)
	s.dice.Load([]int{20, 5, 0, 0, 0, 0})

	s.w.DoViolence(5)

	s.Require().True(s.p.CurrentHealth() == 1 && s.w.occupancy.RoomOfPlayer(s.p) == s.w.DeathRoom, "the player died and revived")
	s.Assert().Empty(s.said())
}
