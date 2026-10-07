package world

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type quaffSuite struct {
	worldTestSuite
	clock time.Time
}

func TestQuaffSuite(t *testing.T) { suite.Run(t, new(quaffSuite)) }

func (s *quaffSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
}

func (s *quaffSuite) do(cmd command.Command) {
	s.T().Helper()
	s.r.Clear()
	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(cmd)))
}

func (s *quaffSuite) carry(name, quaff string, power int) *object.Instance {
	s.T().Helper()
	d := object.NewDefinition(name, name, "wrathrock", rules.ObjectCategoryPotion, nil,
		"a "+name, "A "+name+" is here.", rules.SlotNone, rules.ArmorTypeNone, nil)
	d.Quaff = quaff
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = power
	s.Require().NoError(s.p.Inventory().Add(inst))
	return inst
}

func (s *quaffSuite) failed() event.ResultCode {
	s.T().Helper()
	return sent[event.Failed](s.T(), s.r, 0).Code
}

// the potion's ability on the drinker, at the potion's power, for no mana --
// and the potion is gone
func (s *quaffSuite) TestQuaff_heals() {
	draught := s.carry("draught", "heal", 2)
	s.p.TakeMeleeDamage(50)
	mana := s.p.CurrentMana()

	s.do(command.Quaff{Target: "draught"})

	s.Assert().Equal(event.Quaffed{Actor: "testdood", Item: "a draught"}, sent[event.Quaffed](s.T(), s.r, 0))
	s.Assert().Equal(event.Healed{Actor: "testdood", Target: "testdood", Amount: 14}, sent[event.Healed](s.T(), s.r, 1))
	_, still := s.p.Inventory().Get(draught.Id)
	s.Assert().False(still, "drunk")
	s.Assert().Equal(mana, s.p.CurrentMana(), "a potion costs no mana")
	s.Assert().True(s.p.ReadyAt("heal").IsZero(), "nor the ability's own cooldown")
}

// no gear needed: the potion is the grant
func (s *quaffSuite) TestQuaff_needsNoGear() {
	s.carry("draught", "heal", 1)
	s.Require().Empty(s.p.Equipment().Abilities())
	s.do(command.Quaff{Target: "draught"})
	sent[event.Quaffed](s.T(), s.r, 0)
}

// one potion every rules.QuaffCooldown, and a refused one isn't drunk
func (s *quaffSuite) TestQuaff_cooldown() {
	s.carry("draught", "heal", 1)
	second := s.carry("draught", "heal", 1)
	s.do(command.Quaff{Target: "draught"})

	s.clock = s.clock.Add(rules.QuaffCooldown - time.Second)
	s.do(command.Quaff{Target: "draught"})
	s.Assert().Equal(event.NotReady, s.failed())
	_, still := s.p.Inventory().Get(second.Id)
	s.Assert().True(still, "not drunk")

	s.clock = s.clock.Add(time.Second)
	s.do(command.Quaff{Target: "draught"})
	sent[event.Quaffed](s.T(), s.r, 0)
}

func (s *quaffSuite) TestQuaff_refusals() {
	s.do(command.Quaff{})
	s.Assert().Equal(event.NoTarget, s.failed())

	s.do(command.Quaff{Target: "draught"})
	s.Assert().Equal(event.TargetNotFound, s.failed())

	pelt := s.carry("pelt", "", 1)
	s.do(command.Quaff{Target: "pelt"})
	s.Assert().Equal(event.NotDrinkable, s.failed())
	_, still := s.p.Inventory().Get(pelt.Id)
	s.Assert().True(still)
}

// fine in a fight: that's what they're for
func (s *quaffSuite) TestQuaff_inAFight() {
	s.carry("draught", "heal", 1)
	target, exists := s.w.StartRoom.FindMobile("target")
	s.Require().True(exists)
	s.Require().NoError(s.w.fightLedger.Fight(s.p, target))
	s.p.TakeMeleeDamage(50)
	s.do(command.Quaff{Target: "draught"})
	sent[event.Healed](s.T(), s.r, 1)
}
