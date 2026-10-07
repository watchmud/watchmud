package world

import (
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/player"
)

// grammarWords are what a player types to mean something other than a player:
// parseTarget's "all", the words for yourself, the corpse every mob leaves,
// and what the renderer calls someone it can't name. A character called any
// of them would be targeted by accident, or impersonate the game.
var grammarWords = []string{
	"all", "self", "me", "myself", "you", "corpse",
	"someone", "somebody", "something", "nobody",
	// a line the game starts with a word a name could be: "Nothing you're
	// wearing lets you recall" -- an emote by a Nothing would be it
	"nothing",
}

// IsReservedName says whether a character may not be called this, because the
// world already uses the word. Only asked of names nobody has yet: a mob added
// to content later mustn't lock out the player who had the name first.
func (w *World) IsReservedName(name string) bool {
	return w.reservedNames[player.NameKey(name)]
}

// reservedNames is the grammar plus every word that targets a mob -- its name
// and its aliases, which is exactly what mobile.Definition.Matches accepts --
// so "kill rabbit" never has to choose between a rabbit and a Rabbit.
func reservedNames(c *loader.Content) map[string]bool {
	reserved := make(map[string]bool)
	for _, w := range grammarWords {
		reserved[w] = true
	}
	for _, z := range c.Zones {
		for _, d := range z.MobileDefinitions {
			reserved[player.NameKey(d.Name)] = true
			for _, a := range d.Aliases {
				reserved[player.NameKey(a)] = true
			}
		}
	}
	return reserved
}
