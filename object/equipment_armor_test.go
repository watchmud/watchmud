package object

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/rules"
)

// What armor is worth, and what it therefore argues for. Separate from
// EquipmentSuite because these need a catalog with a real armor table, where
// the test catalog's is deliberately empty.
type EquipmentArmorSuite struct {
	suite.Suite
	eq *Equipment
}

func TestEquipmentArmorSuite(t *testing.T) {
	suite.Run(t, new(EquipmentArmorSuite))
}

func (s *EquipmentArmorSuite) SetupTest() {
	armor := rules.ArmorTypeContent{
		rules.ArmorTypeCloth: {rules.SlotHead: 0, rules.SlotBody: 1},
		rules.ArmorTypePlate: {rules.SlotHead: 1, rules.SlotBody: 4},
	}
	cat, err := rules.NewCatalog(rules.MudTime{}, nil, rules.NewTestRoles(), armor)
	s.Require().NoError(err)
	s.eq = NewEquipment(cat)
}

// wear an armor piece of this type in this slot
func (s *EquipmentArmorSuite) wear(slot rules.EquipmentSlot, name string, t rules.ArmorType) *Instance {
	inst := MakeTestArmor(s.T(), slot, name, t, 100)
	s.eq.Equip(slot, inst)
	return inst
}

func (s *EquipmentArmorSuite) TestNoArmorIsBaseArmorClass() {
	s.Assert().Equal(rules.BaseArmorClass, s.eq.ArmorClass())
}

func (s *EquipmentArmorSuite) TestArmorClassSumsWhatIsWorn() {
	s.wear(rules.SlotHead, "iron helmet", rules.ArmorTypePlate)
	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate)

	s.Assert().Equal(rules.BaseArmorClass+1+4, s.eq.ArmorClass())
}

// The slot decides the number, not just the material: plate on a head is
// worth less than plate on a chest.
func (s *EquipmentArmorSuite) TestSlotDecidesTheWeight() {
	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate)
	s.Assert().Equal(rules.BaseArmorClass+4, s.eq.ArmorClass())

	s.eq.Unequip(rules.SlotBody)
	s.wear(rules.SlotHead, "iron helmet", rules.ArmorTypePlate)
	s.Assert().Equal(rules.BaseArmorClass+1, s.eq.ArmorClass())
}

// A slot the table doesn't mention, and gear that isn't armor at all, are
// worth nothing rather than an error.
func (s *EquipmentArmorSuite) TestUntabledSlotAndNonArmorAreWorthNothing() {
	s.wear(rules.SlotFeet, "plate boots", rules.ArmorTypePlate) // no feet row
	s.wear(rules.SlotWield, "knife", rules.ArmorTypeNone)

	s.Assert().Equal(rules.BaseArmorClass, s.eq.ArmorClass())
	s.Assert().Empty(s.eq.RoleWeights())
}

// The protection is the argument: the same number that raises AC is what
// makes you a Tank, with nobody typing "tank": 4 anywhere.
func (s *EquipmentArmorSuite) TestArmorArguesForTheArmorRole() {
	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate)

	s.Assert().Equal(map[string]int{"tank": 4}, s.eq.RoleWeights())
}

func (s *EquipmentArmorSuite) TestArmorWeightsAccumulate() {
	s.wear(rules.SlotHead, "iron helmet", rules.ArmorTypePlate)
	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate)

	s.Assert().Equal(map[string]int{"tank": 5}, s.eq.RoleWeights())
}

// Cloth on a head is worth 0, so it argues for nothing -- and shows up in no
// listing either, rather than as "cloth cap 0".
func (s *EquipmentArmorSuite) TestWorthlessArmorArguesForNothing() {
	s.wear(rules.SlotHead, "cloth cap", rules.ArmorTypeCloth)

	s.Assert().Empty(s.eq.RoleWeights())
	s.Assert().Empty(s.eq.RoleContributions())
}

// Armor that also declares weights by hand gets both: a magic helmet can
// argue for something plate alone wouldn't.
func (s *EquipmentArmorSuite) TestDeclaredWeightsStackWithDerivedOnes() {
	inst := s.wear(rules.SlotHead, "blessed helm", rules.ArmorTypePlate)
	inst.Definition.RoleWeights = map[string]int{"healer": 2}

	s.Assert().Equal(map[string]int{"tank": 1, "healer": 2}, s.eq.RoleWeights())
}

// Contributions come back in slot order, and the reasons add up to the totals
// -- that is the whole reason they come off one list.
func (s *EquipmentArmorSuite) TestContributionsAreInSlotOrderAndAddUp() {
	s.wear(rules.SlotHead, "iron helmet", rules.ArmorTypePlate)
	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate)

	contributions := s.eq.RoleContributions()
	s.Require().Len(contributions, 2)
	s.Assert().Equal("iron helmet", contributions[0].Instance.Definition.Name)
	s.Assert().Equal("plate mail", contributions[1].Instance.Definition.Name)

	totals := make(map[string]int)
	for _, c := range contributions {
		totals[c.RoleId] += c.Weight
	}
	s.Assert().Equal(s.eq.RoleWeights(), totals)
}

// No role is marked from_armor: armor protects you and argues for nothing.
func (s *EquipmentArmorSuite) TestNoArmorRoleMeansNoDerivedWeight() {
	cat, err := rules.NewCatalog(rules.MudTime{},
		nil,
		[]*rules.Role{{Id: "tank", Name: "Tank"}},
		rules.ArmorTypeContent{rules.ArmorTypePlate: {rules.SlotBody: 4}})
	s.Require().NoError(err)
	s.eq = NewEquipment(cat)

	s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate)

	s.Assert().Equal(rules.BaseArmorClass+4, s.eq.ArmorClass())
	s.Assert().Empty(s.eq.RoleWeights())
}
