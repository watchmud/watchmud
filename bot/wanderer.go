package bot

import (
	"context"
	"regexp"
	"strings"
)

// A wanderer's rules of thumb, beside the adventurer's.
const (
	wanderSteps = 20 // steps between fresh looks, so Run gets a turn
	lingerOneIn = 4  // it stops a while in one room in this many
)

// door is one exit from one room, by the room's name and the direction typed.
type door struct{ from, dir string }

// keepOut is the doors a wanderer never takes: what's past them would kill a
// power-1 bot that only fights back. South of the Edge of the Old Wood is the
// wolves, and behind them the Overturned Oak and the whole Barrow; west of the
// Millpond is the Drowned Mill.
// TestKeepOut_safe walks the real content without these and fails if anything
// aggressive above power 2 is still in reach.
var keepOut = []door{
	{"The Edge of the Old Wood", "south"},
	{"The Millpond", "west"},
}

// roomBlockRe is any room description: the name, an optional indented
// description, and the exits line -- what a player sees on arriving anywhere.
var roomBlockRe = regexp.MustCompile(`(?m)^([^\n]+)\n(?: [^\n]*\n)?\[ Exits: ([^\]\n]*) \]$`)

// wander walks wherever the exits go, a step at a time, lingering now and
// then. It fights only what attacks it and loots nothing: fight loots only a
// ground's prey, and a wanderer has no ground.
func (a *Adventurer) wander(ctx context.Context) (stateFn, error) {
	_, m, err := a.ask("look", roomBlockRe)
	if err != nil {
		return nil, err
	}
	a.here = m[1]
	exits, came := parseExits(m[2]), ""
	for range wanderSteps {
		if next, err := a.mend(ctx); next != nil || err != nil {
			return next, err
		}
		if a.rng.IntN(lingerOneIn) == 0 {
			if err := a.idle(ctx, a.span(a.cfg.Pace.Linger)); err != nil {
				return nil, err
			}
			if next, err := a.mend(ctx); next != nil || err != nil {
				return next, err
			}
		}
		dir := a.pickExit(exits, came)
		if dir == "" {
			a.log("no way on from %s", a.here)
			return a.goHome, nil
		}
		if err := a.pause(ctx, a.cfg.Pace.Walk); err != nil {
			return nil, err
		}
		_, m, err := a.ask(dir, roomBlockRe)
		if err != nil {
			return nil, err
		}
		a.here, exits, came = m[1], parseExits(m[2]), dir
		a.count(func(st *Stats) { st.Steps++ })
		a.say(momentWander)
	}
	return a.wander, nil
}

// mend fights back if attacked and rests if hurt, and says where to go if
// that ended in a death.
func (a *Adventurer) mend(ctx context.Context) (stateFn, error) {
	if a.attacked {
		if err := a.fight(ctx); err != nil {
			return nil, err
		}
	}
	if !a.died && a.needsRest() {
		if err := a.rest(ctx); err != nil {
			return nil, err
		}
	}
	if a.died {
		return a.dead, nil
	}
	return nil, nil
}

// pickExit chooses where to go next from here: never through a keepOut door,
// and never straight back unless there is no other way.
func (a *Adventurer) pickExit(exits []string, came string) string {
	var open []string
	for _, dir := range exits {
		if !kept(a.here, dir) {
			open = append(open, dir)
		}
	}
	if len(open) > 1 {
		back := opposite[came]
		var onward []string
		for _, dir := range open {
			if dir != back {
				onward = append(onward, dir)
			}
		}
		open = onward
	}
	if len(open) == 0 {
		return ""
	}
	return open[a.rng.IntN(len(open))]
}

func kept(room, dir string) bool {
	for _, d := range keepOut {
		if d.from == room && d.dir == dir {
			return true
		}
	}
	return false
}

// parseExits reads "North, East, Up" into what's typed to take them.
func parseExits(s string) []string {
	var out []string
	for _, e := range strings.Split(s, ",") {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" && e != "none" {
			out = append(out, e)
		}
	}
	return out
}
