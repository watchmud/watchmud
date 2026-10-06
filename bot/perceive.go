package bot

import (
	"regexp"
	"strings"
)

// What a bot reads out of a chunk. Every pattern is text telnet/render.go
// writes; when the rendering changes, these are what should notice.

var (
	tellRe     = regexp.MustCompile(`(?m)^([A-Z][a-z]+) tells you, "(.*)"\.$`)
	attackedRe = regexp.MustCompile(`(?m)^.+ (?:hits you for \d+ damage|misses you)\.$`)
	blowRe     = regexp.MustCompile(`(?m)^(?:.+ (?:hits you for \d+ damage|misses you)|You (?:hit|miss) .+)\.$`)
	deathRe    = regexp.MustCompile(`(?m)^(.+) is dead!$`)
	hereRe     = regexp.MustCompile(`(?m)^([A-Z][a-z]+) is here\.$`)
	gotRe      = regexp.MustCompile(`(?m)^You get .+ from .+\.$`)
	coinsRe    = regexp.MustCompile(`^You get \d+ coins? from `)
	powerRe    = regexp.MustCompile(`(?m)^Equipment \(power (\d+)\)`)
	considerRe = regexp.MustCompile(`\(power (\d+); you are (\d+)\)`)
)

const (
	youDied = "You are dead!"
	youFled = "You flee head over heels."
)

// tell is one tell to this bot: who from, and what they said.
type tell struct{ who, text string }

// tellers is what this bot was told, in order.
func tellers(text string) []tell {
	var out []tell
	for _, m := range tellRe.FindAllStringSubmatch(text, -1) {
		out = append(out, tell{m[1], m[2]})
	}
	return out
}

// attacked: something swung at this bot.
func attacked(text string) bool { return attackedRe.MatchString(text) }

// blows: a swing either way in a fight this bot is in.
func blows(text string) bool { return blowRe.MatchString(text) }

// deaths is who died where this bot could see it.
func deaths(text string) []string {
	var out []string
	for _, m := range deathRe.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

// playersHere is the real players in a room: not this bot, not its siblings.
func playersHere(text, self string, siblings []string) []string {
	var out []string
	for _, m := range hereRe.FindAllStringSubmatch(text, -1) {
		name := m[1]
		if strings.EqualFold(name, self) || isSibling(name, siblings) {
			continue
		}
		out = append(out, name)
	}
	return out
}

func isSibling(name string, siblings []string) bool {
	for _, s := range siblings {
		if strings.EqualFold(s, name) {
			return true
		}
	}
	return false
}

// looted is how many things one "get all from corpse" took. Coins go in the
// purse, not the pack, so they aren't things to carry to the donation room.
func looted(text string) int {
	n := 0
	for _, line := range gotRe.FindAllString(text, -1) {
		if !coinsRe.MatchString(line) {
			n++
		}
	}
	return n
}
