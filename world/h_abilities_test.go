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

type handleAbilitiesSuite struct {
	worldTestSuite
	clock time.Time
}

func TestHandleAbilitiesSuite(t *testing.T) {
	suite.Run(t, new(handleAbilitiesSuite))
}

func (s *handleAbilitiesSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	s.clock = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.w.now = func() time.Time { return s.clock }
}

func (s *handleAbilitiesSuite) list() event.Abilities {
	s.T().Helper()
	s.r.Sent = nil
	s.w.handleAbilities(s.handlerParameter(command.Abilities{}), command.Abilities{})
	return sent[event.Abilities](s.T(), s.r, 0)
}

func (s *handleAbilitiesSuite) TestNothing() {
	s.Assert().Empty(s.list().Granted)
}

func (s *handleAbilitiesSuite) TestGrantedAndCooling() {
	d := object.NewTestDefinition(s.T(), "censer", rules.SlotHold, rules.ObjectCategoryOther, rules.ArmorTypeNone)
	d.Abilities = []string{"heal"}
	inst := object.NewInstance(uuid.New(), d)
	inst.Power = 3
	s.Require().NoError(s.p.Inventory().Add(inst))
	s.p.Equipment().Equip(rules.SlotHold, inst)

	want := event.GrantedAbility{Name: "heal", Mana: 20, Cooldown: 10 * time.Second, Item: "censer", Power: 3}
	s.Assert().Equal([]event.GrantedAbility{want}, s.list().Granted)

	s.p.StartCooldown("heal", s.clock.Add(4*time.Second))
	want.ReadyIn = 4 * time.Second
	s.Assert().Equal([]event.GrantedAbility{want}, s.list().Granted)
}
