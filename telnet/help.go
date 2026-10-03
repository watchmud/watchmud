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
		{"look in <corpse>", "what it's holding", []string{"look"}},
		{"consider <mob>", "how a fight with it would go (con)", []string{"consider", "con"}},
	}},
	{"Things", []helpEntry{
		{"get <item>", "pick it up; get all, get 2.knife", []string{"get"}},
		{"get <item> from <corpse>", "loot", []string{"get"}},
		{"drop <item>", "put it down", []string{"drop"}},
		{"wear <item>, wield <item>", "put it on, take up a weapon", []string{"wear", "wield"}},
		{"remove <item>", "take it off", []string{"remove"}},
		{"repair <item>", "mend worn gear, at the smithy; repair all", []string{"repair"}},
		{"list, buy <item>", "the General Store's stock, and buying it", []string{"list", "buy"}},
		{"sell <item>, value <item>", "sell it there (sell all.pelt), or ask first", []string{"sell", "value"}},
		{"inventory", "what you're carrying (i)", []string{"inventory", "i"}},
		{"equipment", "what you're wearing (eq)", []string{"equipment", "eq"}},
	}},
	{"Fighting", []helpEntry{
		{"kill <mob>", "start a fight", []string{"kill"}},
		{"flee", "get out of one", []string{"flee"}},
		{"cast <ability> [player]", "use what your gear grants: cast heal bob (c)", []string{"cast", "c"}},
	}},
	{"You", []helpEntry{
		{"stat", "health and power", []string{"stat"}},
		{"role", "what your gear makes you, and why", []string{"role"}},
		{"color [on|off]", "ANSI color, on or off; just color switches it", []string{"color"}},
	}},
	{"Talking", []helpEntry{
		{"say <words>", "to the room (')", []string{"say", "'"}},
		{"tell <who> <words>", "to one player, anywhere", []string{"tell"}},
		{"shout <words>", "to everyone playing", []string{"shout"}},
		{"who", "who's playing", []string{"who"}},
		{"quit", "save and leave", []string{"quit"}},
	}},
}

// helpTopic is what "help <topic>" answers, for what isn't a command.
type helpTopic struct {
	what string // its line in the command list
	text string
}

var helpTopics = map[string]helpTopic{
	"bots": {"who the [bot] characters are",
		"The characters marked [bot] in 'who' are programs, not people. They hunt\n" +
			"the Hollowfields, rest when they're hurt, and give what they find to the\n" +
			"donation room, east of Temple Square. They leave any hunting ground a\n" +
			"player is using, and they can't chat: a tell gets you an automatic answer.\n"},
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
