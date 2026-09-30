package rules

import "time"

// PowerBand is the range of power a zone is built for -- how content says
// "this is the 10-15 area". Zero value is the bottom band. See LEVELS.md.
type PowerBand struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// Tuning placeholders, all of them -- see the table in LEVELS.md, and move
// them into a rules/*.json together once there is something to tune against.
const (
	// PowerDeltaClamp is how far out of your league anything can be: past
	// this, more difference changes nothing.
	PowerDeltaClamp = 10
	// PowerHitPointsPerTwo is the to-hit bonus for every two points of power
	// difference -- half a point each.
	PowerHitPointsPerTwo = 1
	// PowerDamagePercent is how much each point of power difference moves
	// damage, up or down.
	PowerDamagePercent = 5
)

// PowerDelta is the attacker's power less the defender's, clamped to
// PowerDeltaClamp either way. Combat reads the difference, never the level:
// d20 against AC has to stay bounded, or by power 30 nothing would connect.
func PowerDelta(attacker, defender int) int {
	return max(-PowerDeltaClamp, min(PowerDeltaClamp, attacker-defender))
}

// PowerHitModifier is what the power difference adds to a d20 attack roll.
// Truncated toward zero, so it's symmetric: three above is worth exactly what
// three below costs.
func PowerHitModifier(attacker, defender int) int {
	return PowerDelta(attacker, defender) * PowerHitPointsPerTwo / 2
}

// PowerDamage scales a blow's damage by the power difference, rounded to the
// nearest point. A blow that did anything still does at least one: being
// outclassed makes you weak, not harmless.
func PowerDamage(damage, attacker, defender int) int {
	if damage <= 0 {
		return damage
	}
	percent := 100 + PowerDelta(attacker, defender)*PowerDamagePercent
	return max(1, (damage*percent+50)/100)
}

// Loot power: a drop comes out at the mob's power, with a small chance of a
// bit more. Placeholders, like the rest -- and these set the pace of the whole
// game, since they're the only way power climbs. LEVELS.md.
const (
	LootBumpTwoPercent = 2  // chance of +2
	LootBumpOnePercent = 10 // chance of +1, after that
)

// CorpseDecay is how long a corpse lasts, loot and all, before it crumbles:
// long enough to finish the fight and loot it, short enough that a grind
// doesn't fill the room. A placeholder; LEVELS.md.
const CorpseDecay = 5 * time.Minute

// DroppedDecay is how long something a character drops lasts on the floor
// before it crumbles: long enough for someone to find a donation, short
// enough that a room somebody -- or a bot -- keeps dropping things in turns
// over. Zone resets and wizard loads never go through drop, so what they put
// down stays. A placeholder; LEVELS.md.
const DroppedDecay = 30 * time.Minute

// LootPowerBump is what a d100 roll (0-99) adds to a drop's power.
func LootPowerBump(roll int) int {
	switch {
	case roll < LootBumpTwoPercent:
		return 2
	case roll < LootBumpTwoPercent+LootBumpOnePercent:
		return 1
	default:
		return 0
	}
}
