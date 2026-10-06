package spaces

import (
	"strings"

	"github.com/watchmud/watchmud/lock"
)

// Door is what can stand in an exit: a name to call it by and a lock.Lock for
// whether it's shut. Both sides of an exit share one -- the loader pairs them --
// so opening it from either room opens it for both.
type Door struct {
	Name    string
	Aliases []string
	*lock.Lock
	// Sides are the rooms it stands between, for a reset to see who's there.
	Sides []*Room
}

// occupied is whether a player is in a room on either side of it.
func (d *Door) occupied() bool {
	for _, r := range d.Sides {
		if len(r.Players()) > 0 {
			return true
		}
	}
	return false
}

func NewDoor(name string, aliases []string, l *lock.Lock) *Door {
	return &Door{Name: name, Aliases: aliases, Lock: l}
}

// Matches is whether a player could mean this door by target: its name, any
// word of it, or an alias. "door" is anyone's door; FindDoor decides when
// that's enough.
func (d *Door) Matches(target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	if target == strings.ToLower(d.Name) {
		return true
	}
	for _, w := range strings.Fields(strings.ToLower(d.Name)) {
		if w == target {
			return true
		}
	}
	for _, a := range d.Aliases {
		if strings.ToLower(a) == target {
			return true
		}
	}
	return false
}
