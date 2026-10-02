package rules

// RegenPercent is how much of their max health anyone not fighting gets back
// each regen pulse. A placeholder until playtesting says otherwise -- see
// LEVELS.md, "Tuning Placeholders"
const RegenPercent = 5

// RegenAmount is one pulse's worth for something with this much max health.
// At least a point, or a 2hp rabbit would never heal: 5% of 2 is 0.
func RegenAmount(maxHealth int) int {
	return max(1, maxHealth*RegenPercent/100)
}

// ManaRegenPercent is how much of their max mana everyone gets back each
// regen pulse -- fighting or not, unlike health. A placeholder: LEVELS.md,
// "tuning placeholders"
const ManaRegenPercent = 5

// ManaRegenAmount is one pulse's worth, at least a point.
func ManaRegenAmount(maxMana int) int {
	return max(1, maxMana*ManaRegenPercent/100)
}
