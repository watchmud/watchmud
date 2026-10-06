package bot

import (
	"regexp"
	"strings"
)

// topic is one thing a socialite can answer: the words that ask about it,
// and what it says. Every answer is a fact about the game as it is -- the
// commands in single quotes are checked against the parser by
// TestFAQ_commandsParse, so one that stops existing fails make check.
type topic struct {
	words  []string
	answer string
}

// faq is in order: the first topic a question has a word for is the answer,
// so the narrower topics come before the broad ones.
var faq = []topic{
	{[]string{"recall", "home", "lost", "stuck", "town"},
		"'recall' takes you home to Temple Square from anywhere -- the temple token around your neck lets you, once a minute, though not in the middle of a fight."},
	{[]string{"repair", "repairs", "broken", "broke", "smith", "smithy", "durability", "worn"},
		"Gear wears down in fights and when you die. The smithy, west of Market Square, mends it for coins: 'repair all'."},
	{[]string{"give", "trade", "share", "hand", "lend"},
		"'give <name> to' someone in the same room hands it over, and coins go the same way: 'give 20 coins to' them. What you're wearing stays on.",
	},
	{[]string{"shop", "shops", "store", "buy", "sell", "coins", "coin", "money", "gold"},
		"The General Store is east of Market Square: 'list' shows what's for sale, then 'buy' and 'sell'. Coins come off what you kill."},
	{[]string{"heal", "healing", "health", "hp", "hurt", "rest", "mana", "regen"},
		"Health comes back on its own while you aren't fighting, mana even while you are. Hold a censer and you can 'cast heal'."},
	{[]string{"cast", "spell", "spells", "ability", "abilities", "magic", "skill", "skills"},
		"What you can cast comes from what you wear, and 'abilities' lists it. Censers heal, some weapons smite, some armor provokes."},
	{[]string{"role", "roles", "class", "classes", "tank", "healer", "striker"},
		"There are no classes here: your gear decides your role. 'role' shows what you are and which pieces say so."},
	{[]string{"die", "died", "dead", "death", "dying"},
		"Dying wakes you in Temple Square with a single point of health, and everything you wore takes some wear. There's no corpse to run back for."},
	{[]string{"gear", "equipment", "armor", "armour", "weapon", "weapons", "wear", "wield", "free"},
		"'equipment' shows what you have on and its power. The donation room, east of Temple Square, has things other players left behind: help yourself."},
	{[]string{"bot", "bots"},
		"Bots are listed apart in 'who'. Some hunt the fields, some wander, and I stand here answering questions. 'help bots' says more."},
	{[]string{"door", "doors", "locked", "lock", "key", "keys", "unlock", "shut", "gate", "grate", "chest", "chests", "box"},
		"A closed door shows as '(closed)' among the exits: 'open <door>' opens it, by its name or its direction. Chests open the same way. A locked one needs its key in your pack -- 'unlock' it first. Keys turn up on whoever guards them, or tucked away somewhere nearby.",
	},
	{[]string{"bag", "bags", "satchel", "satchels", "sack", "storage"},
		"The General Store, east of Market Square, sells a leather satchel that holds ten things. 'put <name> in satchel' and 'get <name> from satchel'; 'look in satchel' shows what's in it. Coins stay in your purse.",
	},
	{[]string{"examine", "inspect", "identify", "stats", "compare", "better", "best"},
		"'look <name>' tells you what a thing is: its power, its condition, its armor or damage, and what it lets you cast. It works on what you carry, what's on the floor, and on people and creatures too.",
	},
	{[]string{"next", "harder", "stronger", "outgrown", "mill", "barrow", "boss"},
		"Outgrown the farms? The Drowned Mill, west of the millpond, is for power 4 to 7, and its miller is a fight for two. The Sunken Barrow, under the oak deep in the Old Wood, is for 6 and up -- go as a group.",
	},
	{[]string{"hunt", "hunting", "fight", "fighting", "kill", "level", "levels", "xp", "exp", "monsters", "mobs", "go", "start", "begin", "new", "newbie"},
		"The Hollowfields are south: through the market, out the south gate, down the trail. The rats, geese and beetles on the farms are the easiest; 'consider <name>' tells you if you'd win."},
	{[]string{"help", "commands", "command"},
		"'help' lists every command. Or ask me about hunting, recall, healing, gear, repair, shops or roles."},
}

// menu is the answer when nothing in a question matches.
const menu = "Ask me about hunting, recall, healing, gear, repair, shops, roles or dying -- or try 'help'."

var wordRe = regexp.MustCompile(`[a-z]+`)

// topicFor is the topic a question asks about, or nil.
func topicFor(question string) *topic {
	words := map[string]bool{}
	for _, w := range wordRe.FindAllString(strings.ToLower(question), -1) {
		words[w] = true
	}
	for i := range faq {
		for _, w := range faq[i].words {
			if words[w] {
				return &faq[i]
			}
		}
	}
	return nil
}

// answer is what a socialite says to a question: the topic's answer, or the
// menu if it has none.
func answer(question string) string {
	if t := topicFor(question); t != nil {
		return t.answer
	}
	return menu
}
