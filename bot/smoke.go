package bot

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Config is who the smoke test logs in as. The character must already exist:
// the bot never creates one, because a name, once taken, is taken for good.
type Config struct {
	Name, Password string
}

// Result is a pass. Notes are what it couldn't test, for reasons that are the
// world's rather than the deploy's.
type Result struct {
	Notes []string
}

// NoGoose is the note for a millpond somebody else has already cleared.
const NoGoose = "no goose at the millpond, so the fight wasn't tested"

// How long a step waits for its answer. Variables so the tests can run out of
// patience faster. A fight gets longer: a round a second, 8hp of goose, and up
// to ten seconds before an aggressive one notices you.
var (
	stepTimeout  = 10 * time.Second
	fightTimeout = 90 * time.Second
)

// lineStart matches the start of a line -- or just after a prompt, since the
// bot doesn't echo what it types and the reply lands on the prompt's line.
const lineStart = `(?m)^(?:<\d+/\d+hp> )?`

// prompt is the server's prompt, which ends every reply.
const prompt = `<\d+/\d+hp> `

// gooseInRoom is how a goose shows in a room description.
const gooseInRoom = "An angry goose lowers its neck and hisses at you."

// route is Temple Square to the millpond, one room name per step.
var route = []struct{ dir, room string }{
	{"south", "Market Square"},
	{"south", "South Gate"},
	{"south", "Outside South Gate"},
	{"south", "southern path"},
	{"south", "The Waystone"},
	{"west", "The Millpond"},
}

// Smoke is the deploy check: log in as cfg.Name, recall to Temple Square,
// walk to the millpond, fight whatever geese are there, loot the newest
// corpse, drop what it looted, and recall again before it quits. Every pattern is text the game
// prints, so a change to the rendering or to the route is what it catches.
// It writes one line per step to log, with how long the step took.
func Smoke(c *Client, cfg Config, log io.Writer) (Result, error) {
	var res Result
	geese := 0
	steps := []struct {
		name string
		do   func() error
	}{
		{"connect", func() error {
			_, err := c.Expect(`Welcome to WatchMUD`, stepTimeout)
			return err
		}},
		{"login", func() error { return login(c, cfg) }},
		{"recall", func() error { return recall(c) }},
		{"walk", func() (err error) {
			geese, err = walk(c)
			return err
		}},
		{"fight", func() error {
			if geese == 0 {
				res.Notes = append(res.Notes, NoGoose)
				return nil
			}
			return fight(c, geese)
		}},
		{"loot", func() error {
			if geese == 0 {
				return nil
			}
			return loot(c)
		}},
		// quit from Temple Square, not the millpond: the character logs back in
		// where it quit, and next deploy the geese will have respawned and be
		// waiting -- a fight started before its first recall, which a fight
		// refuses, is a good deploy failing at random
		{"quit", func() error {
			if err := recall(c); err != nil {
				return err
			}
			if err := c.Send("quit"); err != nil {
				return err
			}
			return c.ExpectClosed(stepTimeout)
		}},
	}
	for _, s := range steps {
		start := time.Now()
		if err := s.do(); err != nil {
			fmt.Fprintf(log, "%-8s FAILED after %s\n", s.name, time.Since(start).Round(time.Millisecond))
			return res, fmt.Errorf("%s: %w", s.name, err)
		}
		fmt.Fprintf(log, "%-8s ok  %s\n", s.name, time.Since(start).Round(time.Millisecond))
	}
	return res, nil
}

// login answers the name and password prompts, and never the creation
// question: an unknown name is a failure.
func login(c *Client, cfg Config) error {
	if _, err := c.Expect(`By what name do you wish to be known\? `, stepTimeout); err != nil {
		return err
	}
	if err := c.Send(cfg.Name); err != nil {
		return err
	}
	m, err := c.Expect(`(Password: |No one by the name of|is already playing|Names are 3 to 16|belongs to something else)`, stepTimeout)
	if err != nil {
		return err
	}
	switch m[1] {
	case "Password: ":
	case "No one by the name of":
		return fmt.Errorf("there's no character called %s: create it by hand first -- the bot never creates one", cfg.Name)
	default:
		return fmt.Errorf("the name %s was refused: %q", cfg.Name, m[1])
	}
	if err := c.SendSecret(cfg.Password); err != nil {
		return err
	}
	m, err = c.Expect(`(\[ Exits: |Wrong password\.|is already playing)`, stepTimeout)
	if err != nil {
		return err
	}
	switch m[1] {
	case "[ Exits: ":
		return nil
	case "Wrong password.":
		return fmt.Errorf("wrong password for %s", cfg.Name)
	default:
		return fmt.Errorf("%s is already playing", cfg.Name)
	}
}

// recall goes back to Temple Square.
func recall(c *Client) error {
	if err := c.Send("recall"); err != nil {
		return err
	}
	_, err := room(c, "Temple Square")
	return err
}

// room waits for the description of the named room and returns what follows
// the name, up to the prompt: the description, exits, and whatever is there.
func room(c *Client, name string) (string, error) {
	m, err := c.Expect(lineStart+regexp.QuoteMeta(name)+`\n((?s:.*?))`+prompt, stepTimeout)
	if err != nil {
		return "", err
	}
	return m[1], nil
}

// walk follows the route and counts the geese in the last room.
func walk(c *Client) (int, error) {
	var last string
	for _, r := range route {
		if err := c.Send(r.dir); err != nil {
			return 0, err
		}
		body, err := room(c, r.room)
		if err != nil {
			return 0, fmt.Errorf("going %s to %s: %w", r.dir, r.room, err)
		}
		last = body
	}
	return strings.Count(last, gooseInRoom), nil
}

// fight starts on the geese -- they may already have started on it; the
// server says so and nothing changes -- and waits for every one to die.
func fight(c *Client, geese int) error {
	if err := c.Send("kill goose"); err != nil {
		return err
	}
	for i := range geese {
		m, err := c.Expect(`(angry goose is dead!|You are dead!)`, fightTimeout)
		if err != nil {
			return fmt.Errorf("goose %d of %d: %w", i+1, geese, err)
		}
		if m[1] == "You are dead!" {
			return errors.New("the goose won")
		}
	}
	return nil
}

// loot empties the newest corpse and drops what came out of it, so the bot
// carries nothing from one deploy to the next. Only feathers: the character's
// starting kit stays with it.
func loot(c *Client) error {
	if err := c.Send("get all from corpse"); err != nil {
		return err
	}
	m, err := c.Expect(`(You get .+ from the corpse of angry goose\.|There's nothing in there\.)`, stepTimeout)
	if err != nil {
		return err
	}
	if strings.HasPrefix(m[1], "There's nothing") {
		return nil // the feather is a 60% drop
	}
	if err := c.Send("drop all.feather"); err != nil {
		return err
	}
	_, err = c.Expect(`Dropped\.`, stepTimeout)
	return err
}
