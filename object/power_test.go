package object

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/rules"
)

// A player's power is the average of what they have on. See LEVELS.md.
type PowerSuite struct {
	suite.Suite
	eq *Equipment
}

func TestPowerSuite(t *testing.T) {
	suite.Run(t, new(PowerSuite))
}

func (s *PowerSuite) SetupTest() {
	cat, err := rules.NewTestCatalog()
	s.Require().NoError(err)
	s.eq = NewEquipment(cat)
}

// wear something of this power in this slot, able to take 10 points of wear
func (s *PowerSuite) wear(t *testing.T, slot rules.EquipmentSlot, name string, power int) *Instance {
	t.Helper()
	inst := MakeTestArmor(t, slot, name, rules.ArmorTypeCloth, 10)
	inst.Power = power
	s.eq.Equip(slot, inst)
	return inst
}

func (s *PowerSuite) TestNothingEquippedIsZero() {
	s.Assert().Equal(0, s.eq.Power())
}

func (s *PowerSuite) TestOneItemIsItsPower() {
	s.wear(s.T(), rules.SlotHead, "hat", 12)
	s.Assert().Equal(12, s.eq.Power())
}

func (s *PowerSuite) TestAveragesWhatIsWorn() {
	s.wear(s.T(), rules.SlotHead, "hat", 10)
	s.wear(s.T(), rules.SlotBody, "shirt", 20)
	s.wear(s.T(), rules.SlotFeet, "boots", 30)
	s.Assert().Equal(20, s.eq.Power())
}

// Empty slots don't count: a player wearing only a power-20 ring is power 20,
// not power 20 divided by every slot there is. LEVELS.md, "Known risk".
func (s *PowerSuite) TestEmptySlotsDontCount() {
	s.wear(s.T(), rules.SlotHead, "hat", 20)
	s.wear(s.T(), rules.SlotBody, "shirt", 20)
	s.eq.Unequip(rules.SlotBody)
	s.Assert().Equal(20, s.eq.Power())
}

// Rounds down: one better piece in a set nudges the average, but the number a
// player sees moves when the set as a whole has moved.
func (s *PowerSuite) TestRoundsDown() {
	s.wear(s.T(), rules.SlotHead, "hat", 10)
	s.wear(s.T(), rules.SlotBody, "shirt", 11)
	s.Assert().Equal(10, s.eq.Power())
}

// Broken gear counts for nothing, as it does for AC and role -- and it doesn't
// count as a zero either, which would punish wearing it over wearing nothing.
func (s *PowerSuite) TestBrokenGearDoesntCount() {
	s.wear(s.T(), rules.SlotHead, "hat", 10)
	shirt := s.wear(s.T(), rules.SlotBody, "shirt", 30)
	shirt.Damage(shirt.Durability)
	s.Require().True(shirt.Broken())

	s.Assert().Equal(10, s.eq.Power())
}

func (s *PowerSuite) TestAllBrokenIsZero() {
	hat := s.wear(s.T(), rules.SlotHead, "hat", 10)
	hat.Damage(hat.Durability)
	s.Assert().Equal(0, s.eq.Power())
}
