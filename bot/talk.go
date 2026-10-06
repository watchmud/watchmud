package bot

// moment is when a bot might say something.
type moment int

const (
	momentKill moment = iota
	momentTown
	momentRest
	momentDonate
	momentWander // a step taken, by a wanderer
)

// phrases is what each bot says, keyed by its lowercase name. A bot with no
// entry is a quiet one, not a broken one. How often any of it is said is the
// adventurer's business: talkOneIn and talkEvery.
var phrases = map[string]map[moment][]string{
	"wren": {
		momentKill:   {"Another one for the pile.", "That'll teach it."},
		momentTown:   {"Back in town. Anything exciting happen?", "Wrathrock, sweet Wrathrock."},
		momentRest:   {"Give me a minute.", "Ow."},
		momentDonate: {"Somebody might want these.", "Donation room's got a few new things."},
		momentWander: {"Just stretching my legs.", "Nice day for a walk."},
	},
	"pim": {
		momentKill:   {"Ha!", "Too slow!"},
		momentTown:   {"Home again, home again.", "Who's buying?"},
		momentRest:   {"Just catching my breath...", "That goose bites harder than it looks."},
		momentDonate: {"Free stuff in the donation room, folks.", "Help yourselves."},
		momentWander: {"Where does this go, I wonder?", "Onward!"},
	},
	"odo": {
		momentKill:   {"Rest easy, little beast.", "Sorry about that."},
		momentTown:   {"The fields are quiet today.", "Good to see the square busy."},
		momentRest:   {"Hm. That stung.", "Sitting down for a bit."},
		momentDonate: {"Leaving these for whoever needs them.", "For the new ones."},
		momentWander: {"I like it out here.", "Mind if I pass through?"},
	},
}
