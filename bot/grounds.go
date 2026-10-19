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

// The farms -- rats, geese, beetles and dogs -- are for new characters and a
// bot still buying its kit: they stop at power 2. A bot in the whole store kit
// is power 3 and hunts the bandit track and the boar wood instead, never
// further: south of the Edge of the Old Wood are the wolves, power 5 and
// aggressive, which a bot that grows to 4 would call a fair fight, and south
// of the Woodcutter's Clearing is the hedge-witch, who is never prey.
// TestGrounds_safe holds every route to that. Two grounds to each band so a
// player on one sends the bots to the other, instead of back to town to wait.
var hollowfieldsRoute = []step{
	{"south", "Market Square"},
	{"south", "South Gate"},
	{"south", "Outside South Gate"},
	{"south", "southern path"},
	{"south", "The Waystone"},
}

var grounds = []ground{{
	name: "the west fields",
	// 0, not the zone's 1: broken gear stops counting toward power, and a bot
	// whose kit has all worn out is power 0. It should still hunt rats.
	minPower: 0,
	maxPower: 2,
	route:    hollowfieldsRoute,
	patrol: []step{
		{"west", "The Millpond"},
		{"south", "The Overgrown Orchard"},
		{"south", "The Red Barn"},
		{"north", "The Overgrown Orchard"},
		{"north", "The Millpond"},
		{"east", "The Waystone"},
	},
	prey: []prey{
		{"rat", "field rat", "A fat field rat noses through the straw."},
		{"goose", "angry goose", "An angry goose lowers its neck and hisses at you."},
		{"beetle", "giant beetle", "A giant beetle the size of a dog clicks its mandibles."},
	},
	loot: []string{"pelt", "feather", "carapace"},
}, {
	name:     "the east fields",
	minPower: 0,
	maxPower: 2,
	route:    hollowfieldsRoute,
	patrol: []step{
		{"east", "Along the Hedgerow"},
		{"south", "The Wheat Field"},
		{"west", "Farm Lane"},
		{"north", "The Waystone"},
	},
	prey: []prey{
		{"rat", "field rat", "A fat field rat noses through the straw."},
		{"dog", "wild dog", "A mangy wild dog watches you, hackles up."},
	},
	loot: []string{"pelt"},
}, {
	name:     "the bandit track",
	minPower: 3,
	maxPower: 5,
	route:    hollowfieldsRoute,
	patrol: []step{
		{"east", "Along the Hedgerow"},
		{"south", "The Wheat Field"},
		{"south", "A Narrow Track"},
		{"east", "The Bandit Camp"},
		{"west", "A Narrow Track"},
		{"north", "The Wheat Field"},
		{"west", "Farm Lane"},
		{"north", "The Waystone"},
	},
	prey: []prey{
		{"lookout", "bandit lookout", "A bandit lookout leans on a stick, pretending not to watch the path."},
		// "thug", not "bandit": the lookout answers to bandit too, and a bandit
		// that fled can end up standing beside it
		{"thug", "bandit", "A bandit in a patched jerkin fingers a cudgel."},
	},
	loot: []string{"hood", "cudgel"},
}, {
	name:     "the boar wood",
	minPower: 3,
	maxPower: 5,
	route:    hollowfieldsRoute,
	patrol: []step{
		{"south", "Farm Lane"},
		{"south", "The Old Bridge"},
		{"south", "The Edge of the Old Wood"},
		{"west", "The Woodcutter's Clearing"},
		{"east", "The Edge of the Old Wood"},
		{"north", "The Old Bridge"},
		{"north", "Farm Lane"},
		{"north", "The Waystone"},
	},
	prey: []prey{
		{"boar", "wild boar", "A wild boar roots at the ground, tusks muddy."},
	},
	// the jerkin answers to "boar"; "jerkin" is the quilted one's too
	loot: []string{"boar"},
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
