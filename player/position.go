package player

// Position is how a player is: on their feet, or less so. In memory only --
// everyone logs in standing.
type Position int

const (
	Standing Position = iota
	Sitting
	Resting
	Sleeping
)

// String is the word for it in a room description: "" for standing, which is
// the usual and goes unsaid.
func (p Position) String() string {
	switch p {
	case Sitting:
		return "sitting"
	case Resting:
		return "resting"
	case Sleeping:
		return "sleeping"
	}
	return ""
}

// RegenPercent is how much faster health and mana come back in this
// position, as a percentage of standing's. Placeholders, like regen itself.
func (p Position) RegenPercent() int {
	switch p {
	case Sitting:
		return 150
	case Resting:
		return 200
	case Sleeping:
		return 300
	}
	return 100
}

func (p *Player) Position() Position      { return p.position }
func (p *Player) SetPosition(to Position) { p.position = to }
