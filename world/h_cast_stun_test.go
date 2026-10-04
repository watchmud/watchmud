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
	"github.com/watchmud/watchmud/testdice"
)

// stun in testcontent: 20 mana, 30s cooldown, 2 rounds. The start room has a
// Target Drone and a Little Drone.
type stunSuite struct {
	worldTestSuite
	clock  time.Time
	target *mobile.Instance
	little *mobile.Instance
	pulse  rules.PulseCount
}

func TestStunSuite(t *testing.T) {
	suite.Run(t, new(stunSuite))
}

func (s *stunSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
	var found bool
	s.target, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.little, found = s.w.StartRoom.FindMobile("little")
	s.Require().True(found)
	s.pulse = 10
	s.wieldMace()
}

func (s *stunSuite) wieldMace() {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), "mace", rules.SlotWield, rules.ObjectCategoryWeapon, rules.ArmorTypeNone)
	d.Damage = "1d8"
	d.Abilities = []string{"stun"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 1
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotWield, inst)
}

func (s *stunSuite) stun(target string) {
	s.T().Helper()
	s.r.Clear()
	cmd := command.Cast{Ability: "stun", Target: target}
	s.w.handleCast(s.handlerParameter(cmd), cmd)
}

// mobSwings runs one round with only the mob swinging -- the player's side of
// the fight is ended, so its dice don't get in the way -- and answers
// whether it swung. A d20 of 1 misses, so nothing else is rolled.
func (s *stunSuite) mobSwings(mob *mobile.Instance) bool {
	s.T().Helper()
	s.w.fightLedger.EndFight(s.p)
	dice := testdice.New()
	dice.Load([]int{1})
	s.w.roller = dice
	s.r.Clear()
	s.pulse += 1
	s.w.DoViolence(s.pulse)
	return heardStruck(s.r, mob.Name())
}

func (s *stunSuite) TestStun() {
	s.stun("target")

	s.Assert().Equal(event.Stunned{Actor: "testdood", Target: "Target Drone", Rounds: 2},
		sent[event.Stunned](s.T(), s.r, 0))
	s.Assert().Equal(25, s.target.CurHealth, "no damage")
	s.Assert().Equal(80, s.p.CurrentMana())
	s.Assert().Equal(s.clock.Add(30*time.Second), s.p.ReadyAt("stun"))
}

// two rounds skipped, each one seen, then it swings again
func (s *stunSuite) TestSkipsTwoSwings() {
	s.stun("target")

	s.Assert().False(s.mobSwings(s.target))
	s.Assert().Equal(event.Staggered{Name: "Target Drone"}, sent[event.Staggered](s.T(), s.r, 0))
	s.Assert().False(s.mobSwings(s.target))
	s.Assert().True(s.mobSwings(s.target), "and then it swings")
}

// a skipped round is a used round: no swing banked for later
func (s *stunSuite) TestUsesTheRound() {
	s.stun("target")

	s.mobSwings(s.target)

	s.Assert().Equal(s.pulse, s.w.fightLedger.GetFight(s.target).LastPulse)
}

// it opens a fight, like smite
func (s *stunSuite) TestStartsAFight() {
	s.stun("target")

	s.Assert().Equal(s.target, s.w.fightLedger.GetFight(s.p).Fightee)
	s.Assert().Equal(s.p, s.w.fightLedger.GetFight(s.target).Fightee)
}

// stunning something else mid-fight keeps your swings where they were
func (s *stunSuite) TestMidFightKeepsYourTarget() {
	s.Require().NoError(s.w.startFight(s.p, s.little))

	s.stun("target")

	s.Assert().Equal(s.little, s.w.fightLedger.GetFight(s.p).Fightee)
	s.Assert().Equal(2, s.w.fightLedger.Stunned(s.target))
}

// a stunned mob that's provoked is still stunned: the stun is on the mob
func (s *stunSuite) TestSurvivesATurn() {
	other := player.NewTestPlayer(uuid.New(), "otherdood", &player.Recorder{})
	s.w.PlacePlayer(other, s.w.StartRoom)
	s.stun("target")

	s.w.fightLedger.Turn(s.target, other)

	s.Assert().False(s.mobSwings(s.target))
}

// a stun gone with its mob: a fresh one from the next reset swings
func (s *stunSuite) TestEndsWithDeath() {
	s.stun("target")

	s.w.combatantDied(s.target, s.w.StartRoom)

	s.Assert().Equal(0, s.w.fightLedger.Stunned(s.target))
}

func (s *stunSuite) TestNoFoe() {
	s.stun("")

	s.Assert().Equal(event.NoFoe, sent[event.Failed](s.T(), s.r, 0).Code)
	s.Assert().Equal(100, s.p.CurrentMana())
}
