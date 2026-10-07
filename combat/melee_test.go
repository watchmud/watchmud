package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/dice"
	"github.com/watchmud/watchmud/testdice"
)

type MeleeSuite struct {
	suite.Suite
	fighter Combatant
	victim  Combatant
	roller  *testdice.LoadedDice
}

func TestMeleeSuite(t *testing.T) {
	suite.Run(t, new(MeleeSuite))
}

func (s *MeleeSuite) SetupTest() {
	s.fighter = NewTestCombatant("fighter", 10, []DamageType{}, []DamageType{})
	s.victim = NewTestCombatant("victim", 10, []DamageType{}, []DamageType{})
	s.roller = testdice.New()
}

func (s *MeleeSuite) TestHitFailed() {
	s.roller.Add(2)

	noChance, err := AttemptMeleeAttack(s.roller, s.fighter, s.victim)

	s.Assert().NoError(err)
	s.Assert().False(noChance.WasHit)
	s.Assert().Equal(0, noChance.Damage)
}

func (s *MeleeSuite) TestHitSuccess() {
	s.roller.Load([]int{19, 3})
	result, err := AttemptMeleeAttack(s.roller, s.fighter, s.victim)
	s.Assert().NoError(err)
	s.Assert().True(result.WasHit)
	s.Assert().Equal(3, result.Damage)
}

// The boundary, in both directions. A hit is the roll *meeting* armor class,
// so AC 10 is hit by a 10 and missed by a 9 -- the one-off that a doc comment
// reading "> victim's AC" would talk somebody into "fixing".
func (s *MeleeSuite) TestRollEqualToArmorClassHits() {
	s.roller.Load([]int{10, 4}) // victim is AC 10
	result, err := AttemptMeleeAttack(s.roller, s.fighter, s.victim)
	s.Require().NoError(err)
	s.Assert().True(result.WasHit)
	s.Assert().Equal(4, result.Damage)
}

func (s *MeleeSuite) TestRollOneUnderArmorClassMisses() {
	s.roller.Load([]int{9})
	result, err := AttemptMeleeAttack(s.roller, s.fighter, s.victim)
	s.Require().NoError(err)
	s.Assert().False(result.WasHit)
	s.Assert().Equal(0, result.Damage)
}

// Armor class is the whole of the defense: the same roll that lands on an
// unarmored victim misses an armored one.
func (s *MeleeSuite) TestArmorClassIsWhatDecidesIt() {
	armored := NewTestCombatant("armored", 15, []DamageType{}, []DamageType{})

	s.roller.Load([]int{14, 4})
	onUnarmored, err := AttemptMeleeAttack(s.roller, s.fighter, s.victim)
	s.Require().NoError(err)

	s.roller.Load([]int{14, 4})
	onArmored, err := AttemptMeleeAttack(s.roller, s.fighter, armored)
	s.Require().NoError(err)

	s.Assert().True(onUnarmored.WasHit, "14 beats AC 10")
	s.Assert().False(onArmored.WasHit, "14 does not reach AC 15")
}

// A defender with no armor class at all cannot be missed, since a d20 has no
// roll below 1. Nothing should be able to reach combat in that state -- the
// loader defaults a mob without an "ac" to rules.BaseArmorClass for exactly
// this reason -- but the arithmetic is worth pinning down.
func (s *MeleeSuite) TestArmorClassZeroCannotBeMissed() {
	unmissable := NewTestCombatant("unmissable", 0, []DamageType{}, []DamageType{})

	s.roller.Load([]int{1, 2})
	result, err := AttemptMeleeAttack(s.roller, s.fighter, unmissable)
	s.Require().NoError(err)
	s.Assert().True(result.WasHit)
}

// TODO critical success
// TODO critical fail
// TODO resistance
// TODO vulnerability

// Power: the attacker ten above gets +5 to hit and half again the damage.
// A 5 wouldn't hit AC 10 on its own.
func (s *MeleeSuite) TestPowerAboveHelpsYouHitAndHurt() {
	s.fighter.(*TestCombatant).SetPower(10)
	s.roller.Load([]int{5, 4})

	result, err := AttemptMeleeAttack(s.roller, s.fighter, s.victim)
	s.Require().NoError(err)
	s.Assert().True(result.WasHit)
	s.Assert().Equal(6, result.Damage)
}

// Ten below: a 14 is a 9 against AC 10, a miss.
func (s *MeleeSuite) TestPowerBelowMakesYouMiss() {
	s.victim.(*TestCombatant).SetPower(10)
	s.roller.Load([]int{14})

	result, err := AttemptMeleeAttack(s.roller, s.fighter, s.victim)
	s.Require().NoError(err)
	s.Assert().False(result.WasHit)
}

// And when you do land one, it does half.
func (s *MeleeSuite) TestPowerBelowHalvesYourDamage() {
	s.victim.(*TestCombatant).SetPower(10)
	s.roller.Load([]int{15, 4})

	result, err := AttemptMeleeAttack(s.roller, s.fighter, s.victim)
	s.Require().NoError(err)
	s.Assert().True(result.WasHit)
	s.Assert().Equal(2, result.Damage)
}

// sharp hits for 1d2-3: always below zero, before the clamp
type sharp struct{ *TestCombatant }

func (sharp) WeaponDamageRoll() string { return "1d2-3" }

// a blow never heals, whatever the dice say
func TestCalculateDamage_neverNegative(t *testing.T) {
	roller := dice.New([32]byte{})
	fighter := sharp{NewTestCombatant("fighter", 10, []DamageType{}, []DamageType{})}
	victim := NewTestCombatant("victim", 10, []DamageType{}, []DamageType{})
	for range 20 {
		damage, err := calculateDamage(roller, fighter, victim)
		require.NoError(t, err)
		assert.Zero(t, damage)
	}
}
