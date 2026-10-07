package rules

import (
	"errors"
	"fmt"
	"strings"
)

// Direction is a compass direction, plus up and down.
//
// The zero value, DirectionNone, means "unset" and is not a usable direction.
type Direction int

const (
	DirectionNone Direction = iota
	DirectionNorth
	DirectionEast
	DirectionSouth
	DirectionWest
	DirectionUp
	DirectionDown
)

// ErrUnknownDirection is returned by the parse functions. Callers can test
// for it with errors.Is; the returned error also names the offending input.
var ErrUnknownDirection = errors.New("unknown direction")

// The single source of truth. Indexing by the constants keeps the table and
// the constants from drifting apart, and the compiler sizes the array.
var table = [...]struct {
	name   string
	abbrev string
}{
	DirectionNone:  {"None!", ""},
	DirectionNorth: {"North", "n"},
	DirectionEast:  {"East", "e"},
	DirectionSouth: {"South", "s"},
	DirectionWest:  {"West", "w"},
	DirectionUp:    {"Up", "u"},
	DirectionDown:  {"Down", "d"},
}

// AllUsableDirections is every usable direction, in const order
var AllUsableDirections = []Direction{DirectionNorth, DirectionEast, DirectionSouth, DirectionWest, DirectionUp, DirectionDown}

// Index returns the index of d in AllUsableDirections, or -1 if d is not valid.
func (d Direction) Index() int {
	for i := range AllUsableDirections {
		if d == AllUsableDirections[i] {
			return i
		}
	}
	return -1

}

// Valid reports whether d is a usable direction. DirectionNone is not.
func (d Direction) Valid() bool {
	return d >= DirectionNorth && d <= DirectionDown
}

// String returns the display name, e.g. "North"
func (d Direction) String() string {
	if d < 0 || int(d) >= len(table) {
		return fmt.Sprintf("Direction(%d)", int32(d))
	}
	return table[d].name
}

// Abbrev returns the one-letter form, e.g. "n". Empty for DirectionNone.
func (d Direction) Abbrev() string {
	if !d.Valid() {
		return ""
	}
	return table[d].abbrev
}

// ParseDirection resolves a direction exactly: either the full name or the one-letter
// abbreviation, case-insensitively. Use this for world files, config, and
// anywhere a typo should be an error rather than a guess.
func ParseDirection(s string) (Direction, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	for _, d := range AllUsableDirections {
		if s == table[d].abbrev || s == strings.ToLower(table[d].name) {
			return d, nil
		}
	}
	return DirectionNone, fmt.Errorf("%w: %q", ErrUnknownDirection, s)
}

// ParseDirectionPrefix resolves a direction from player input, accepting any unambiguous
// prefix of a direction name: "n", "no", "nort" and "north" all give DirectionNorth.
// Use this for command parsing, never for files.
func ParseDirectionPrefix(s string) (Direction, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return DirectionNone, fmt.Errorf("%w: empty", ErrUnknownDirection)
	}
	for _, d := range AllUsableDirections {
		if strings.HasPrefix(strings.ToLower(table[d].name), s) {
			return d, nil
		}
	}
	return DirectionNone, fmt.Errorf("%w: %q", ErrUnknownDirection, s)
}

// FormatDirection renders a set of directions for display:
// "North, DirectionSouth, DirectionEast, DirectionDown", or "None!" when empty.
func FormatDirection(dirs []Direction) string {
	if len(dirs) == 0 {
		return table[DirectionNone].name
	}
	names := make([]string, 0, len(dirs))
	for _, d := range dirs {
		names = append(names, d.String())
	}
	return strings.Join(names, ", ")
}

func (d *Direction) UnmarshalText(text []byte) error {
	v, err := ParseDirection(string(text))
	if err != nil {
		return err
	}
	*d = v
	return nil
}

func (d Direction) MarshalText() ([]byte, error) {
	if !d.Valid() {
		return nil, fmt.Errorf("%w: %d", ErrUnknownDirection, int32(d))
	}
	return []byte(strings.ToLower(table[d].name)), nil
}

// Opposite is the way back: North for South, Up for Down. DirectionNone has none.
func (d Direction) Opposite() Direction {
	switch d {
	case DirectionNorth:
		return DirectionSouth
	case DirectionSouth:
		return DirectionNorth
	case DirectionEast:
		return DirectionWest
	case DirectionWest:
		return DirectionEast
	case DirectionUp:
		return DirectionDown
	case DirectionDown:
		return DirectionUp
	}
	return DirectionNone
}
