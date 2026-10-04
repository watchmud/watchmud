package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
)

// provoke in testcontent: 15 mana, 15s cooldown, no amount. The start room
// has a Target Drone and a Little Drone.
type provokeSuite struct {
	worldTestSuite
	clock  time.Time
	target *mobile.Instance
	little *mobile.Instance
	other  *player.Player
}

func TestProvokeSuite(t *testing.T) {
	suite.Run(t, new(provokeSuite))
}

func (s *provokeSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
	var found bool
	s.target, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.little, found = s.w.StartRoom.FindMobile("little")
	s.Require().True(found)
	s.other = player.NewTestPlayer(uuid.New(), "otherdood", &player.Recorder{})
	s.w.PlacePlayer(s.other, s.w.StartRoom)
	s.wearPlate()
}

// wear a breastplate granting provoke
func (s *provokeSuite) wearPlate() {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), "breastplate", rules.SlotBody, rules.ObjectCategoryArmor, rules.ArmorTypePlate)
	d.Abilities = []string{"provoke"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotBody, inst)
}

func (s *provokeSuite) provoke(target string) {
	s.T().Helper()
	s.r.Clear()
	cmd := command.Cast{Ability: "provoke", Target: target}
	s.w.handleCast(s.handlerParameter(cmd), cmd)
}

func (s *provokeSuite) targetOf(c *mobile.Instance) any {
	s.T().Helper()
	fight := s.w.fightLedger.GetFight(c)
	s.Require().NotNil(fight, "%s isn't fighting", c.Name())
	return fight.Fightee
}

// the mob was fighting someone else: now it's fighting you
func (s *provokeSuite) TestTakesTheMobBack() {
	s.Require().NoError(s.w.startFight(s.other, s.target))

	s.provoke("target")

	s.Assert().Equal(s.p, s.targetOf(s.target))
	s.Assert().Equal(s.target, s.w.fightLedger.GetFight(s.p).Fightee, "and you're fighting it")
	s.Assert().Equal(s.target, s.w.fightLedger.GetFight(s.other).Fightee, "they still are too")
	s.Assert().Equal(event.Provoked{Actor: "testdood", Target: "Target Drone"},
		sent[event.Provoked](s.T(), s.r, 0))
	s.Assert().Equal(85, s.p.CurrentMana())
	s.Assert().Equal(s.clock.Add(15*time.Second), s.p.ReadyAt("provoke"))
}

// with nobody fighting it, provoke is a pull: a fight both ways, like kill
func (s *provokeSuite) TestPulls() {
	s.provoke("target")

	s.Assert().Equal(s.p, s.targetOf(s.target))
	s.Assert().Equal(s.target, s.w.fightLedger.GetFight(s.p).Fightee)
}

// you're fighting one thing and provoke another: it turns on you, but your
// swings stay where they were
func (s *provokeSuite) TestMidFightKeepsYourTarget() {
	s.Require().NoError(s.w.startFight(s.p, s.little))
	s.Require().NoError(s.w.startFight(s.other, s.target))

	s.provoke("target")

	s.Assert().Equal(s.p, s.targetOf(s.target))
	s.Assert().Equal(s.little, s.w.fightLedger.GetFight(s.p).Fightee)
}

// you're fighting, and provoke something idle: it starts on you
func (s *provokeSuite) TestMidFightIdleMob() {
	s.Require().NoError(s.w.startFight(s.p, s.little))

	s.provoke("target")

	s.Assert().Equal(s.p, s.targetOf(s.target))
	s.Assert().Equal(s.little, s.w.fightLedger.GetFight(s.p).Fightee)
}

// bare provoke means whoever you're fighting
func (s *provokeSuite) TestBareProvoke() {
	s.Require().NoError(s.w.startFight(s.other, s.target))
	s.Require().NoError(s.w.startFight(s.p, s.target))
	s.Require().Equal(s.other, s.targetOf(s.target))

	s.provoke("")

	s.Assert().Equal(s.p, s.targetOf(s.target))
}

// already on you, it's wasted -- and still paid for, like an unneeded heal
func (s *provokeSuite) TestAlreadyOnYou() {
	s.Require().NoError(s.w.startFight(s.p, s.target))

	s.provoke("target")

	s.Assert().Equal(event.Provoked{Actor: "testdood", Target: "Target Drone", Already: true},
		sent[event.Provoked](s.T(), s.r, 0))
	s.Assert().Equal(85, s.p.CurrentMana())
	s.Assert().Equal(s.clock.Add(15*time.Second), s.p.ReadyAt("provoke"))
}

// the turn isn't a free swing: the mob keeps its place in the rounds
func (s *provokeSuite) TestKeepsThePulse() {
	s.Require().NoError(s.w.startFight(s.other, s.target))
	s.w.fightLedger.GetFight(s.target).LastPulse = 42

	s.provoke("target")

	s.Assert().Equal(rules.PulseCount(42), s.w.fightLedger.GetFight(s.target).LastPulse)
}

// once taken, it holds: first-engaged rules from here, until you leave the
// fight -- then it goes back to whoever's been at it longest
func (s *provokeSuite) TestHoldsUntilYouLeave() {
	s.Require().NoError(s.w.startFight(s.other, s.target))
	s.provoke("target")

	s.w.fightLedger.EndAllFightsWith(s.p.Id())

	s.Assert().Equal(s.other, s.targetOf(s.target))
}

func (s *provokeSuite) TestNoFoe() {
	s.provoke("")

	s.Assert().Equal(event.NoFoe, sent[event.Failed](s.T(), s.r, 0).Code)
	s.Assert().Equal(100, s.p.CurrentMana())
}
