package rules

import "time"

// MaxMana is everyone's, flat, the way max health is today. Gear decides
// what you can cast, not how much -- see the abilities spec. If max health
// ever comes from gear, this goes with it.
const MaxMana = 100

// QuaffCooldown is how long after one potion before the next: without it a
// pack of healing draughts is a health bar no mob can empty. Every potion
// shares it. A placeholder; LEVELS.md.
const QuaffCooldown = 10 * time.Second
