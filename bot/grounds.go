package bot

import "strings"

// ground is somewhere to hunt. Hand-written for now: the spec's future work
// replaces these with a map a bot learns by exploring. TestGrounds_walk
// walks every one against the real content, so a content change that breaks
// one fails make check.
type ground struct {
	name               string
	minPower, maxPower int
	route              []step // from Temple Square to where the patrol starts
	patrol             []step // a loop: it ends where it starts
	prey               []prey
	loot               []string // keywords of what the prey drop: what a donation drops
}

// step is one move and the room it should arrive in.
type step struct{ dir, room string }

// prey is a mob worth hunting: what to type, how its death reads, and how it
// looks in a room -- a room shows descriptions, not keywords.
type prey struct{ keyword, name, seen string }

var grounds = []ground{{
	name: "the Hollowfields",
	// 0, not the zone's 1: broken gear stops counting toward power, and a bot
	// whose kit has all worn out is power 0. It should still hunt rats.
	minPower: 0,
	maxPower: 5,
	route: []step{
		{"south", "Market Square"},
		{"south", "South Gate"},
		{"south", "Outside South Gate"},
		{"south", "southern path"},
		{"south", "The Waystone"},
	},
	// the farms: not the bandit track, the wood or the witch's hollow
	patrol: []step{
		{"west", "The Millpond"},
		{"south", "The Overgrown Orchard"},
		{"south", "The Red Barn"},
		{"north", "The Overgrown Orchard"},
		{"east", "Farm Lane"},
		{"east", "The Wheat Field"},
		{"north", "Along the Hedgerow"},
		{"west", "The Waystone"},
	},
	// Never the hedge-witch: her ring and censer are where the first healers
	// come from (LEVELS.md), and a bot farming her would take them from
	// players. consider decides among these; the table only says which are
	// fair game at all.
	prey: []prey{
		{"rat", "field rat", "A fat field rat noses through the straw."},
		{"goose", "angry goose", "An angry goose lowers its neck and hisses at you."},
		{"beetle", "giant beetle", "A giant beetle the size of a dog clicks its mandibles."},
		{"dog", "wild dog", "A mangy wild dog watches you, hackles up."},
	},
	loot: []string{"pelt", "feather", "carapace"},
}}

// preyIn is the kinds of prey in a room's description, once each.
func (g *ground) preyIn(room string) []prey {
	var out []prey
	for _, p := range g.prey {
		if strings.Contains(room, p.seen) {
			out = append(out, p)
		}
	}
	return out
}

// isPrey says whether a death line's name is something this ground hunts.
func (g *ground) isPrey(name string) bool {
	for _, p := range g.prey {
		if p.name == name {
			return true
		}
	}
	return false
}

// homeward is the way off a ground on foot, from patrol step i -- the room
// it just reached -- to the first room of the route: round the loop to its
// start whichever way is shorter, then back up the route. Walking out, rather
// than recalling, is what a player sees as "Wren leaves east." and not a
// crowd blinking out of existence the moment they arrive.
func (g *ground) homeward(i int) []step {
	var out []step
	forward := len(g.patrol) - 1 - i // steps on round to where the loop starts
	if forward <= i+1 {
		out = append(out, g.patrol[i+1:]...)
	} else {
		for j := i; j >= 0; j-- {
			out = append(out, step{opposite[g.patrol[j].dir], g.patrolRoomBefore(j)})
		}
	}
	for k := len(g.route) - 1; k >= 1; k-- {
		out = append(out, step{opposite[g.route[k].dir], g.route[k-1].room})
	}
	return out
}

// patrolRoomBefore is where patrol step j sets out from.
func (g *ground) patrolRoomBefore(j int) string {
	if j == 0 {
		return g.route[len(g.route)-1].room
	}
	return g.patrol[j-1].room
}

// opposite is the way back. Every exit a route or patrol uses must have one,
// which TestGrounds_walk checks against the real content.
var opposite = map[string]string{
	"north": "south", "south": "north",
	"east": "west", "west": "east",
	"up": "down", "down": "up",
}
