package combat

import (
	"log"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/rules"
)

type FightLedgerSuite struct {
	suite.Suite
	fightLedger *FightLedger
}

func TestFightLedgerSuite(t *testing.T) {
	suite.Run(t, new(FightLedgerSuite))
}

func (suite *FightLedgerSuite) SetupTest() {
	suite.fightLedger = NewFightLedger()
}

func (suite *FightLedgerSuite) TestIsFighting() {
	fighter := NewTestCombatant("fighter", 10, []DamageType{}, []DamageType{})
	fightee := NewTestCombatant("fightee", 10, []DamageType{}, []DamageType{})

	log.Printf("%p", fighter)
	log.Printf("%p", fightee)

	suite.Assert().False(suite.fightLedger.IsFighting(fighter))
	suite.Assert().False(suite.fightLedger.IsFighting(fightee))

	suite.fightLedger.Fight(fighter, fightee)

	// fighting starts a bidirectional fight
	suite.Assert().True(suite.fightLedger.IsFighting(fighter))
	suite.Assert().True(suite.fightLedger.IsFighting(fightee))
}

func (suite *FightLedgerSuite) TestAlreadyFighting() {
	fighter := NewTestCombatant("fighter", 10, []DamageType{}, []DamageType{})
	fightee := NewTestCombatant("fightee", 10, []DamageType{}, []DamageType{})

	suite.Assert().NoError(suite.fightLedger.Fight(fighter, fightee))
	// can't fight when you're already fighting
	suite.Assert().Error(suite.fightLedger.Fight(fighter, fightee))
}

func (suite *FightLedgerSuite) TestFightingSomeoneWhoIsFighting() {
	fighter := NewTestCombatant("fighter", 10, []DamageType{}, []DamageType{})
	otherFighter := NewTestCombatant("otherFighter", 10, []DamageType{}, []DamageType{})
	fightee := NewTestCombatant("fightee", 10, []DamageType{}, []DamageType{})

	suite.Assert().NoError(suite.fightLedger.Fight(otherFighter, fightee))
	// can be fought by more than 1
	suite.Assert().NoError(suite.fightLedger.Fight(fighter, fightee))
	suite.Assert().Equal(otherFighter, suite.fightLedger.fightMap[fightee.Id()].Fightee)
}

func (suite *FightLedgerSuite) TestEndFight() {
	fighter := NewTestCombatant("fighter", 10, []DamageType{}, []DamageType{})
	fightee := NewTestCombatant("fightee", 10, []DamageType{}, []DamageType{})

	suite.Assert().NoError(suite.fightLedger.Fight(fighter, fightee))

	suite.fightLedger.EndFight(fighter)
	suite.Assert().False(suite.fightLedger.IsFighting(fighter))
	suite.Assert().True(suite.fightLedger.IsFighting(fightee))
}

func combatant(name string) *TestCombatant {
	return NewTestCombatant(name, 10, []DamageType{}, []DamageType{})
}

// The King kills the tank. He has to turn on whoever else is hitting him
// rather than stand there, still "in a fight", never swinging again.
func (suite *FightLedgerSuite) TestTargetGoneMeansTheNextAttacker() {
	tank, striker, king := combatant("tank"), combatant("striker"), combatant("king")
	suite.Require().NoError(suite.fightLedger.Fight(tank, king))
	suite.Require().NoError(suite.fightLedger.Fight(striker, king))
	suite.Require().Same(tank, suite.fightLedger.GetFight(king).Fightee, "held by whoever engaged first")

	suite.fightLedger.EndAllFightsWith(tank.Id())

	fight := suite.fightLedger.GetFight(king)
	suite.Require().NotNil(fight)
	suite.Assert().Same(striker, fight.Fightee)
}

// With several left, the one who has been at it longest: the same rule that
// kept him on the tank in the first place. Repeated because the ledger is a
// map, and a lucky iteration order would pass this once by accident.
func (suite *FightLedgerSuite) TestTheNextAttackerIsTheEarliest() {
	for range 50 {
		ledger := NewFightLedger()
		tank, first, second, third, king := combatant("tank"), combatant("first"), combatant("second"), combatant("third"), combatant("king")
		for _, c := range []*TestCombatant{tank, first, second, third} {
			suite.Require().NoError(ledger.Fight(c, king))
		}

		ledger.EndAllFightsWith(tank.Id())
		suite.Require().Same(first, ledger.GetFight(king).Fightee)

		ledger.EndAllFightsWith(first.Id())
		suite.Require().Same(second, ledger.GetFight(king).Fightee)
	}
}

// Players too: fighting a rat while a wolf chews on you, the rat dies and you
// turn to the wolf instead of standing there taking it.
func (suite *FightLedgerSuite) TestAPlayerTurnsOnWhateverIsStillBitingThem() {
	player, rat, wolf := combatant("player"), combatant("rat"), combatant("wolf")
	suite.Require().NoError(suite.fightLedger.Fight(player, rat))
	suite.Require().NoError(suite.fightLedger.Fight(wolf, player))

	suite.fightLedger.EndAllFightsWith(rat.Id())

	suite.Require().True(suite.fightLedger.IsFighting(player))
	suite.Assert().Same(wolf, suite.fightLedger.GetFight(player).Fightee)
}

// One on one, nobody is left, so nobody picks anybody up.
func (suite *FightLedgerSuite) TestNobodyLeftMeansNoFight() {
	a, b := combatant("a"), combatant("b")
	suite.Require().NoError(suite.fightLedger.Fight(a, b))

	suite.fightLedger.EndAllFightsWith(a.Id())

	suite.Assert().False(suite.fightLedger.InFight(b))
	suite.Assert().Empty(suite.fightLedger.GetFights())
}

// Turn points a fighter at someone else -- the one way a target is ever
// replaced -- and keeps its pulse, so a turned mob gets no extra swing.
func (suite *FightLedgerSuite) TestTurn() {
	mob := NewTestCombatant("mob", 10, []DamageType{}, []DamageType{})
	first := NewTestCombatant("first", 10, []DamageType{}, []DamageType{})
	tank := NewTestCombatant("tank", 10, []DamageType{}, []DamageType{})
	suite.Require().NoError(suite.fightLedger.Fight(first, mob))
	suite.fightLedger.GetFight(mob).LastPulse = 7

	suite.fightLedger.Turn(mob, tank)

	fight := suite.fightLedger.GetFight(mob)
	suite.Assert().Equal(tank, fight.Fightee)
	suite.Assert().Equal(rules.PulseCount(7), fight.LastPulse, "no free swing")
	suite.Assert().Equal(mob, suite.fightLedger.GetFight(first).Fightee, "first still swings at it")
}

// a mob that wasn't fighting just starts on the one who turned it
func (suite *FightLedgerSuite) TestTurnIdle() {
	mob := NewTestCombatant("mob", 10, []DamageType{}, []DamageType{})
	tank := NewTestCombatant("tank", 10, []DamageType{}, []DamageType{})

	suite.fightLedger.Turn(mob, tank)

	suite.Assert().Equal(tank, suite.fightLedger.GetFight(mob).Fightee)
	suite.Assert().False(suite.fightLedger.IsFighting(tank), "turning is one way")
}
