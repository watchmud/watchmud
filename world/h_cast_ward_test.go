package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/combat"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/rules"
	"github.com/watchmud/watchmud/testdice"
)

// ward in testcontent: 20 mana, 20s cooldown, 10 + 2 per power, for 30s.
// The start room has a Target Drone.
type wardSuite struct {
	worldTestSuite
	clock time.Time
	bob   *player.Player
	bobR  *player.Recorder
	drone *mobile.Instance
	dice  *testdice.LoadedDice
}

func TestWardSuite(t *testing.T) {
	suite.Run(t, new(wardSuite))
}

func (s *wardSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
	s.bobR = &player.Recorder{}
	s.bob = player.NewTestPlayer(uuid.New(), "bob", s.bobR)
	s.w.PlacePlayer(s.bob, s.w.StartRoom)
	var found bool
	s.drone, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.dice = testdice.New()
	s.w.roller = s.dice
	s.wearRing(1)
}

// wear a ring granting ward, at this power
func (s *wardSuite) wearRing(power int) {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), "ring", rules.SlotFingers, rules.ObjectCategoryTreasure, rules.ArmorTypeNone)
	d.Abilities = []string{"ward"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = power
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotFingers, inst)
}

// wear a shirt that a landed blow can wear down
func (s *wardSuite) wearShirt() *object.Instance {
	s.T().Helper()
	d := object.NewTestDefinition(s.T(), "shirt", rules.SlotBody, rules.ObjectCategoryArmor, rules.ArmorTypeLeather)
	d.MaxDurability = 10
	inst := object.NewInstance(uuid.New(), d)
	s.p.Equipment().Equip(rules.SlotBody, inst)
	return inst
}

func (s *wardSuite) ward(target string) {
	s.T().Helper()
	s.r.Clear()
	s.bobR.Clear()
	cmd := command.Cast{Ability: "ward", Target: target}
	s.w.handleCast(s.handlerParameter(cmd), cmd)
}

// struck: the drone swings once at victim, a hit for these rolls (d20, then
// damage), and only the drone swings.
func (s *wardSuite) struck(victim *player.Player) event.Struck {
	s.T().Helper()
	s.dice.Load([]int{20, 5, 0})
	s.w.fightLedger = combat.NewFightLedger()
	s.Require().NoError(s.w.fightLedger.Fight(s.drone, victim))
	s.w.fightLedger.EndFight(victim)
	s.r.Clear()
	s.w.DoViolence(5)
	for _, msg := range s.r.Sent {
		if st, ok := msg.(event.Struck); ok {
			return st
		}
	}
	s.FailNow("no blow")
	return event.Struck{}
}

func (s *wardSuite) wardBroken() bool {
	for _, msg := range s.r.Sent {
		if _, ok := msg.(event.WardBroken); ok {
			return true
		}
	}
	return false
}

func (s *wardSuite) TestWardSelf() {
	s.ward("")

	s.Assert().Equal(event.Warded{Actor: "testdood", Target: "testdood", Amount: 12},
		sent[event.Warded](s.T(), s.r, 0))
	s.Assert().Equal(12, s.p.Ward(s.clock), "10 + 2 per power")
	s.Assert().Equal(80, s.p.CurrentMana())
	s.Assert().Equal(s.clock.Add(20*time.Second), s.p.ReadyAt("ward"))
}

func (s *wardSuite) TestWardOther() {
	s.ward("bob")

	want := event.Warded{Actor: "testdood", Target: "bob", Amount: 12}
	s.Assert().Equal(want, sent[event.Warded](s.T(), s.r, 0))
	s.Assert().Equal(want, sent[event.Warded](s.T(), s.bobR, 0), "bob sees it too")
	s.Assert().Equal(12, s.bob.Ward(s.clock))
	s.Assert().Zero(s.p.Ward(s.clock))
}

func (s *wardSuite) TestOnCooldown() {
	s.ward("")
	s.ward("")

	s.Assert().Equal(event.NotReady, sent[event.Failed](s.T(), s.r, 0).Code)
	s.Assert().Equal(80, s.p.CurrentMana(), "nothing spent")
}

// a ward neither starts a fight nor is refused in one
func (s *wardSuite) TestInAFight() {
	s.ward("")
	s.Assert().False(s.w.fightLedger.InFight(s.p))

	s.clock = s.clock.Add(time.Minute)
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.ward("")
	s.Assert().Equal(12, s.p.Ward(s.clock))
}

// a blow the ward can take all of: no health lost, and no wear on the armor
func (s *wardSuite) TestTakesTheWholeBlow() {
	s.p.SetWard(100, s.clock, time.Minute)
	shirt := s.wearShirt()
	health := s.p.CurrentHealth()

	st := s.struck(s.p)

	s.Assert().True(st.Hit)
	s.Assert().Zero(st.Damage)
	s.Assert().Positive(st.Absorbed)
	s.Assert().Equal(health, s.p.CurrentHealth())
	s.Assert().Equal(100-st.Absorbed, s.p.Ward(s.clock))
	s.Assert().False(s.wardBroken())
	s.Assert().Equal(10, shirt.Durability, "it never reached the shirt")
}

// what the ward can't take gets through, and the ward is gone
func (s *wardSuite) TestBreaks() {
	s.p.SetWard(1, s.clock, time.Minute)
	shirt := s.wearShirt()
	health := s.p.CurrentHealth()

	st := s.struck(s.p)

	s.Assert().Equal(1, st.Absorbed)
	s.Assert().Positive(st.Damage)
	s.Assert().Equal(health-st.Damage, s.p.CurrentHealth())
	s.Assert().Zero(s.p.Ward(s.clock))
	s.Assert().True(s.wardBroken())
	s.Assert().Equal(9, shirt.Durability, "what got through wears the shirt as usual")
	s.Assert().Equal(event.WardBroken{Name: "testdood"}, s.r.Sent[len(s.r.Sent)-1].(event.WardBroken),
		"after the blow")
}

// once its time is up it takes nothing
func (s *wardSuite) TestFades() {
	s.ward("")
	s.clock = s.clock.Add(30 * time.Second)
	health := s.p.CurrentHealth()

	st := s.struck(s.p)

	s.Assert().Zero(st.Absorbed)
	s.Assert().Equal(health-st.Damage, s.p.CurrentHealth())
	s.Assert().False(s.wardBroken(), "a faded ward says nothing")
}
