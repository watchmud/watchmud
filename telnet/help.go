package telnet

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// help is answered by the connection, like a parse error, rather than sent to
// the world: it is a list of what to type, and only parse.go knows that. Each
// entry names the verbs it shows so help_test.go can hold it to the parser --
// a verb help offers and the parser refuses is a new player's first
// "Unknown request". Builder commands stay out; to a player they don't exist.

type helpEntry struct {
	usage string
	what  string
	verbs []string
}

type helpSection struct {
	title   string
	entries []helpEntry
}

var helpSections = []helpSection{
	{"Moving around", []helpEntry{
		{"n s e w u d", "walk that way", []string{"n", "s", "e", "w", "u", "d"}},
		{"exits", "where you can go from here", []string{"exits"}},
		{"recall", "back to the start", []string{"recall"}},
	}},
	{"Looking", []helpEntry{
		{"look, look <thing>", "the room, or something in it (l)", []string{"look", "l"}},
		{"look in <corpse>, track", "what it's holding; which way a mob went",
			[]string{"look", "track"}},
		{"consider <mob>", "how a fight with it would go (con)", []string{"consider", "con"}},
	}},
	{"Things", []helpEntry{
		{"get <item>", "pick it up; get all, get 2.knife", []string{"get"}},
		{"get <item> from <corpse>", "loot, or take from a bag or chest", []string{"get"}},
		{"put <item> in <bag|chest>", "put it away; put all.pelt in satchel", []string{"put"}},
		{"give <item> to <player>", "hand it over; give 20 coins to bob", []string{"give"}},
		{"drop, junk, donate <item>", "put down; destroy; send to the donation room",
			[]string{"drop", "junk", "donate"}},
		{"wear <item>, wield <item>", "put it on, take up a weapon", []string{"wear", "wield"}},
		{"remove <item>", "take it off", []string{"remove"}},
		{"open, close, unlock, lock", "doors and chests, by name or way; keys as needed",
			[]string{"open", "close", "unlock", "lock"}},
		{"repair <item>", "mend worn gear, at the smithy; repair all", []string{"repair"}},
		{"list, buy, sell, value", "at the General Store: its stock; trade; prices",
			[]string{"list", "buy", "sell", "value"}},
		{"inventory, equipment", "what you carry (i), and wear (eq)", []string{"inventory", "i", "equipment", "eq"}},
	}},
	{"Fighting", []helpEntry{
		{"kill <mob>, flee, wimpy", "fight; run; run on your own below some health",
			[]string{"kill", "flee", "wimpy"}},
		{"cast <ability> [target]", "gear's spells: cast heal bob, cast smite goose (c)", []string{"cast", "c"}},
	}},
	{"You", []helpEntry{
		{"sit, rest, sleep, stand", "heal faster off your feet; 'wake' to get up",
			[]string{"sit", "rest", "sleep", "stand", "wake"}},
		{"stat, role, abilities", "you; what your gear makes you; what it casts", []string{"stat", "role", "abilities"}},
		{"color [on|off]", "ANSI color, on or off; just color switches it", []string{"color"}},
	}},
	{"Talking", []helpEntry{
		{"say <words>, emote <act>", "to the room ('), or act it out (:)", []string{"say", "'", "emote", ":"}},
		{"tell <who> <words>, reply", "to one player, anywhere; reply to the last", []string{"tell", "reply"}},
		{"whisper, ask <who> <words>", "to one in the room, player or mob", []string{"whisper", "ask"}},
		{"shout, ooc <words>", "to everyone; ooc is chat, for questions (ooc off)", []string{"shout", "ooc"}},
		{"follow <who>, group, gt", "walk and fight as one; gt talks (assist, split)",
			[]string{"follow", "group", "gt", "gtell", "ungroup", "assist", "split"}},
		{"socials, toggle", "smile, bow, wave and more; what you hear", []string{"socials", "toggle"}},
		{"bug, idea, typo <words>", "tell whoever runs the game", []string{"bug", "idea", "typo"}},
		{"who, where, time", "who's playing; who's near you; the hour", []string{"who", "where", "time"}},
		{"commands, quit", "every command there is; save and leave", []string{"commands", "quit"}},
	}},
}

// helpTopic is what "help <topic>" answers, for what isn't a command.
type helpTopic struct {
	what string // its line in the command list
	text string
}

var helpTopics = map[string]helpTopic{
	"bots": {"who the bots are",
		"The characters listed under Bots in 'who' are programs, not people. Most\n" +
			"hunt the Hollowfields, rest when they're hurt, and give what they find to\n" +
			"the donation room, east of Temple Square; they leave any hunting ground a\n" +
			"player is using. Some just wander, stopping a while here and there: they\n" +
			"fight only what attacks them and take nothing. A socialite stands in\n" +
			"Temple Square welcoming new characters: ask it a question, said or told,\n" +
			"about hunting, recall, healing, gear, repair, shops or roles. The rest\n" +
			"can't chat: a tell gets you an automatic answer.\n"},
}

// helpText is built once: the sections never change while the server runs.
var helpText = func() string {
	var b strings.Builder
	for i, s := range helpSections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(s.title + "\n")
		for _, e := range s.entries {
			fmt.Fprintf(&b, "  %-26s %s\n", e.usage, e.what)
		}
	}
	b.WriteString("\nMore\n")
	for _, name := range slices.Sorted(maps.Keys(helpTopics)) {
		fmt.Fprintf(&b, "  %-26s %s\n", "help "+name, helpTopics[name].what)
	}
	return b.String()
}()

// helpFor answers every way of asking: help, ? or commands for the command
// list, with a word after it for a topic.
func helpFor(line string) (string, bool) {
	words := strings.Fields(strings.ToLower(line))
	if len(words) == 0 {
		return "", false
	}
	switch words[0] {
	case "help", "?", "commands":
	default:
		return "", false
	}
	if len(words) == 1 {
		return helpText, true
	}
	if topic, ok := helpTopics[words[1]]; ok {
		return topic.text, true
	}
	return "There's no help on that. Type 'help' for the commands.\n", true
}

// isHelp is every way of asking, for the name prompt, which explains itself
// rather than answering.
func isHelp(line string) bool {
	_, ok := helpFor(line)
	return ok
}
