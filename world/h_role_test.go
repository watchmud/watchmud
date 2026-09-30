package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/command"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/object"
	"github.com/watchmud/watchmud/rules"
)

type RoleTestSuite struct {
	worldTestSuite
}

func TestRoleTestSuite(t *testing.T) {
	suite.Run(t, new(RoleTestSuite))
}

// equip puts an item straight into a slot, skipping the wear handler: these
// tests are about what equipment adds up to, not about how it got there.
func (s *RoleTestSuite) equip(loc rules.EquipmentSlot, name string, weights map[string]int) {
	d := object.NewDefinition(
		name,
		name,
		"wrathrock",
		rules.ObjectCategoryArmor,
		nil,
		name,
		name+" is here.",
		loc,
		rules.ArmorTypeCloth,
		rules.EmptyObjectBehaviors)
	d.RoleWeights = weights
	s.p.Equipment().Equip(loc, object.NewInstance(uuid.New(), d))
}

func (s *RoleTestSuite) role() event.Role {
	s.w.handleRole(s.handlerParameter(command.Role{}), command.Role{})
	return sent[event.Role](s.T(), s.r, 0)
}

func (s *RoleTestSuite) TestNoEquipmentIsNoRole() {
	e := s.role()
	s.Assert().Equal("", e.Current)
	// every role still shows up, so the player can see where they aren't
	s.Require().Len(e.Standings, 3)
	for _, st := range e.Standings {
		s.Assert().Equal(0, st.Total)
		s.Assert().Empty(st.Sources)
	}
}

func (s *RoleTestSuite) TestEquipmentWithNoRoleWeightIsStillNoRole() {
	s.equip(rules.SlotHead, "party hat", nil)
	s.Assert().Equal("", s.role().Current)
}

func (s *RoleTestSuite) TestArmorMakesYouATank() {
	s.equip(rules.SlotHead, "iron helmet", map[string]int{"tank": 2})
	e := s.role()
	s.Assert().Equal("Tank", e.Current)
	s.Assert().Equal([]string{"iron helmet 2"}, e.Standings[0].Sources)
	s.Assert().Equal(2, e.Standings[0].Total)
}

func (s *RoleTestSuite) TestWeightsAccumulateAcrossSlots() {
	s.equip(rules.SlotHead, "iron helmet", map[string]int{"tank": 2})
	s.equip(rules.SlotBody, "chain shirt", map[string]int{"tank": 3})
	e := s.role()
	s.Assert().Equal("Tank", e.Current)
	s.Assert().Equal(5, e.Standings[0].Total)
	// sources come back in slot order (Head before Body), not map order
	s.Assert().Equal([]string{"iron helmet 2", "chain shirt 3"}, e.Standings[0].Sources)
}

func (s *RoleTestSuite) TestOneItemCanArgueForTwoRoles() {
	s.equip(rules.SlotHold, "brass censer", map[string]int{"healer": 3, "tank": 1})
	e := s.role()
	s.Assert().Equal("Healer", e.Current)
	s.Assert().Equal(1, e.Standings[0].Total) // tank
	s.Assert().Equal(3, e.Standings[1].Total) // healer
	s.Assert().Equal([]string{"brass censer 1"}, e.Standings[0].Sources)
	s.Assert().Equal([]string{"brass censer 3"}, e.Standings[1].Sources)
}

// The whole point of the phase: the role follows the gear, with no command in
// between.
func (s *RoleTestSuite) TestSwappingGearSwapsTheRole() {
	s.equip(rules.SlotHead, "iron helmet", map[string]int{"tank": 2})
	s.Assert().Equal("Tank", s.role().Current)

	s.p.Equipment().Equip(rules.SlotHead, nil)
	s.equip(rules.SlotWield, "long knife", map[string]int{"striker": 4})

	s.w.handleRole(s.handlerParameter(command.Role{}), command.Role{})
	s.Assert().Equal("Striker", sent[event.Role](s.T(), s.r, 1).Current)
}

func (s *RoleTestSuite) TestStatReportsTheSameRole() {
	s.equip(rules.SlotWield, "long knife", map[string]int{"striker": 4})
	s.w.handleStat(s.handlerParameter(command.Stat{}), command.Stat{})
	st := sent[event.Stat](s.T(), s.r, 0)
	s.Assert().Equal("Striker", st.Role)
	s.Assert().Equal("Human", st.Lineage)
	s.Assert().Equal("wrathrock", st.ZoneId)
	s.Assert().Equal("temple_square", st.RoomId)
	s.Assert().Equal(100, st.MaxHealth)
}
