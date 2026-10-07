package combat

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/rules"
)

type MeleeAttackResult struct {
	WasHit     bool
	Damage     int
	DamageType string // TODO should be enum
	// TODO other status affects: staggering, and so on
}

func (result MeleeAttackResult) String() string {
	if result.WasHit {
		return fmt.Sprintf("Hit! %d damage!", result.Damage)
	}
	return fmt.Sprintf("Missed.")
}

// AttemptMeleeAttack calculates the result of a melee attack: roll d20, apply
// the attacker's modifiers, and hit when the result *meets or beats* the
// victim's armor class. Meeting it is a hit -- AC is the number you need, not
// the number you have to exceed -- so AC 10 is hit by a 10.
//
// Both sides of that comparison are absolute and start from
// rules.BaseArmorClass: a player is that plus what they are wearing, a mob is
// whatever its definition says, defaulting to the same baseline. Anything
// that starts measuring armor class from somewhere else is comparing two
// different scales through one operator.
func AttemptMeleeAttack(roller rules.Roller, fighter Combatant, victim Combatant) (MeleeAttackResult, error) {
	roll, err := roller.Roll("1d20")
	if err != nil {
		return MeleeAttackResult{}, err
	}
	log.Trace().Msgf("%s attacks %s, rolls a %d", fighter.Name(), victim.Name(), roll)
	// TODO need impl for critical failure and critical success
	//	criticalFailure := roll == 1
	//	criticalSuccess := roll == 20

	// Power adds on top of the d20, rather than growing AC, so the roll
	// stays bounded however high power climbs. See rules.PowerDelta.
	modifiedRoll := roll + fighter.CalculateMeleeRollModifiers() +
		rules.PowerHitModifier(fighter.Power(), victim.Power())
	wasHit := modifiedRoll >= victim.ArmorClass()
	wasHitStr := "missed."
	damage := 0
	if wasHit {
		wasHitStr = "hit!"
		damage, err = calculateDamage(roller, fighter, victim)
		if err != nil {
			return MeleeAttackResult{}, err
		}
	}
	log.Trace().
		Msgf("%s attacks %s with modified roll of %d vs ac of %d, %s With %d damage.",
			fighter.Name(),
			victim.Name(),
			modifiedRoll,
			victim.ArmorClass(),
			wasHitStr,
			damage)
	return MeleeAttackResult{
		WasHit:     wasHit,
		Damage:     damage,
		DamageType: "TODO",
	}, nil
}

func calculateDamage(roller rules.Roller, fighter Combatant, victim Combatant) (int, error) {
	damage, err := roller.Roll(fighter.WeaponDamageRoll())
	if err != nil {
		return 0, err
	}
	damage = rules.PowerDamage(damage, fighter.Power(), victim.Power())
	modifier := ""

	if victim.HasResistanceTo(fighter.WeaponDamageType()) {
		modifier = " (resistance)"
		damage = damage / 2
	} else if victim.IsVulnerableTo(fighter.WeaponDamageType()) {
		modifier = " (vulnerability)"
		damage = damage * 2
	}
	// a blow never heals: dice like 1d2-3 can roll below zero
	damage = max(damage, 0)
	log.Trace().Msgf(" %s does %d damage to %s%s", fighter.Name(), damage, victim.Name(), modifier)
	return damage, nil
}
