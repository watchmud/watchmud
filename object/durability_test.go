package object

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/rules"
)

type DurabilitySuite struct {
	suite.Suite
	eq *Equipment
}

func TestDurabilitySuite(t *testing.T) {
	suite.Run(t, new(DurabilitySuite))
}

func (s *DurabilitySuite) newCatalog() (*rules.Catalog, error) {
	armor := rules.ArmorTypeContent{
		rules.ArmorTypePlate: {rules.SlotHead: 1, rules.SlotBody: 4},
	}

	return rules.NewCatalog(rules.MudTime{}, nil, rules.NewTestRoles(), armor)
}

func (s *DurabilitySuite) SetupTest() {
	cat, err := s.newCatalog()
	s.Require().NoError(err)
	s.eq = NewEquipment(cat)
}

// gear of this armor type, with this much durability in it
func (s *DurabilitySuite) make(slot rules.EquipmentSlot, name string, t rules.ArmorType, maxDurability int) *Instance {
	return MakeTestArmor(s.T(), slot, name, t, maxDurability)
}

func (s *DurabilitySuite) wear(slot rules.EquipmentSlot, name string, t rules.ArmorType, maxDurability int) *Instance {
	inst := MakeTestArmor(s.T(), slot, name, t, maxDurability)
	s.eq.Equip(slot, inst)
	return inst
}

func (s *DurabilitySuite) TestNewInstanceStartsAtFullDurability() {
	inst := MakeTestArmor(s.T(), rules.SlotBody, "plate mail", rules.ArmorTypePlate, 80)
	s.Assert().Equal(80, inst.Durability)
	s.Assert().True(inst.WearsOut())
	s.Assert().False(inst.Broken())
}

// Everything is indestructible until content says otherwise -- a definition
// with no max is gear that never wears out, not gear that is born broken.
func (s *DurabilitySuite) TestNoMaxDurabilityMeansIndestructible() {
	inst := s.make(rules.SlotBody, "plate mail", rules.ArmorTypePlate, rules.Indestructible)

	s.Assert().False(inst.WearsOut())
	s.Assert().False(inst.Broken())
	s.Assert().False(inst.Damage(1), "damage does nothing")
	s.Assert().False(inst.Broken())
}

func (s *DurabilitySuite) TestDamageCountsDown() {
	inst := s.make(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 3)

	s.Assert().False(inst.Damage(1))
	s.Assert().Equal(2, inst.Durability)
	s.Assert().False(inst.Damage(1))
	s.Assert().Equal(1, inst.Durability)
	s.Assert().False(inst.Broken())
}

// The blow that breaks it says so, and only that one: the caller shouldn't
// have to remember what the state was to know whether to announce it.
func (s *DurabilitySuite) TestBreakingIsReportedOnce() {
	inst := s.make(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 2)

	s.Require().False(inst.Damage(1))
	s.Assert().True(inst.Damage(1), "the blow that finished it")
	s.Assert().True(inst.Broken())
	s.Assert().False(inst.Damage(1), "and not again afterwards")
}

func (s *DurabilitySuite) TestDurabilityNeverGoesNegative() {
	inst := s.make(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 2)
	inst.Damage(10)
	s.Assert().Equal(0, inst.Durability)
}

func (s *DurabilitySuite) TestRepairMakesItNewAgain() {
	inst := s.make(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 5)
	inst.Damage(5)
	s.Require().True(inst.Broken())

	inst.Repair()

	s.Assert().Equal(5, inst.Durability)
	s.Assert().False(inst.Broken())
}

// The whole reason breaking is worth modeling: what you are wearing stops
// protecting you the moment it gives out, without leaving your slot.
func (s *DurabilitySuite) TestBrokenArmorIsWorthNoArmorClass() {
	plate := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 2)
	s.Require().Equal(rules.BaseArmorClass+4, s.eq.ArmorClass())

	plate.Damage(2)

	s.Assert().Equal(rules.BaseArmorClass, s.eq.ArmorClass())
	s.Assert().Equal(plate, s.eq.At(rules.SlotBody), "and it is still being worn")
}

// and stops arguing for the role it was arguing for
func (s *DurabilitySuite) TestBrokenArmorArguesForNothing() {
	plate := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 2)
	s.Require().Equal(map[string]int{"tank": 4}, s.eq.RoleWeights())

	plate.Damage(2)

	s.Assert().Empty(s.eq.RoleWeights())
	s.Assert().Empty(s.eq.RoleContributions())
}

// Hand-declared weights go the same way: a broken censer is not keeping
// anybody upright.
func (s *DurabilitySuite) TestBrokenGearDropsDeclaredWeightsToo() {
	censer := s.wear(rules.SlotHands, "brass censer", rules.ArmorTypeNone, 2)
	censer.Definition.RoleWeights = map[string]int{"healer": 3}
	s.Require().Equal(map[string]int{"healer": 3}, s.eq.RoleWeights())

	censer.Damage(2)

	s.Assert().Empty(s.eq.RoleWeights())
}

// What a blow can wear down: armor, that has durability, that isn't already
// finished.
func (s *DurabilitySuite) TestDamageableArmor() {
	plate := s.wear(rules.SlotBody, "plate mail", rules.ArmorTypePlate, 5)
	s.wear(rules.SlotHands, "brass censer", rules.ArmorTypeNone, 5)                   // not armor
	s.wear(rules.SlotHead, "iron helmet", rules.ArmorTypePlate, rules.Indestructible) // never wears

	broken := s.wear(rules.SlotLegs, "plate greaves", rules.ArmorTypePlate, 1)
	broken.Damage(1)

	s.Assert().Equal([]*Instance{plate}, s.eq.DamageableArmor())
}

func (s *DurabilitySuite) TestNothingWornMeansNothingToWearDown() {
	s.Assert().Empty(s.eq.DamageableArmor())
}
