package bot

import (
	"context"
	"regexp"
	"strings"
	"time"
)

// A socialite's rules of thumb.
const (
	// answerEvery: one answer per person this often, said or told. Long
	// enough that two people can't make it talk in a loop, short enough
	// for a newcomer's next question.
	answerEvery = 20 * time.Second
	maxGreeted  = 1000 // names it remembers greeting; then it starts over
)

var (
	firstTimeRe = regexp.MustCompile(`(?m)^([A-Z][a-z]+) has entered the game for the first time\.$`)
	saysRe      = regexp.MustCompile(`(?m)^([A-Z][a-z]+) says, "(.*)"\.$`)
)

// welcomeFor is what a new character hears from a socialite, once.
func welcomeFor(name string) string {
	return "Welcome to Wrathrock, " + name + "! I'm a bot who answers questions: ask me about hunting, recall, healing or gear, or say 'help'."
}

// socialize stands in Temple Square, where every new character arrives,
// greets each one, and answers what people ask -- said in the room or told.
// It never leaves on its own; a fight or a death brings it back.
func (a *Adventurer) socialize(ctx context.Context) (stateFn, error) {
	if a.here != home {
		return a.goHome, nil
	}
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		ch, err := a.c.ReadChunk(a.cfg.Pace.Poll)
		if err != nil {
			return nil, err
		}
		a.notice(ch)
		if a.died {
			return a.dead, nil
		}
		if a.attacked {
			return a.fightThen(a.goHome), nil
		}
		lines := a.react(ch.Text)
		if err := a.flushTells(); err != nil {
			return nil, err
		}
		for _, line := range lines {
			if err := a.pause(ctx, a.cfg.Pace.Think); err != nil {
				return nil, err
			}
			if _, _, err := a.ask("say "+line, saidRe); err != nil {
				return nil, err
			}
		}
	}
}

// react is what a socialite says to a chunk: a welcome for each new
// character, an answer for each question put to it.
func (a *Adventurer) react(text string) []string {
	var out []string
	for _, m := range firstTimeRe.FindAllStringSubmatch(text, -1) {
		if a.greeted == nil || len(a.greeted) >= maxGreeted {
			a.greeted = map[string]bool{}
		}
		if !a.greeted[m[1]] {
			a.greeted[m[1]] = true
			out = append(out, welcomeFor(m[1]))
		}
	}
	for _, m := range saysRe.FindAllStringSubmatch(text, -1) {
		who, said := m[1], m[2]
		if strings.EqualFold(who, a.cfg.Name) || isSibling(who, a.cfg.Siblings) {
			continue
		}
		if !a.askedMe(said) {
			continue
		}
		if last, ok := a.toldAt[who]; ok && a.now().Sub(last) < answerEvery {
			continue
		}
		a.toldAt[who] = a.now()
		out = append(out, answer(said))
	}
	return out
}

// askedMe: something said in the room is for the socialite if it names it,
// or asks for help, or is a question it has an answer to. A question it has
// no answer to, not put to it by name, is somebody else's conversation.
func (a *Adventurer) askedMe(said string) bool {
	lower := strings.ToLower(said)
	if strings.Contains(lower, strings.ToLower(a.cfg.Name)) || strings.HasPrefix(strings.TrimSpace(lower), "help") {
		return true
	}
	return strings.Contains(said, "?") && topicFor(said) != nil
}
