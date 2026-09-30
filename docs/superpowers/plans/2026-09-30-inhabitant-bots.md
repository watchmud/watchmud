# Inhabitant Bots Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Always-on, openly labelled bots that hunt the Hollowfields, loot, rest, donate what they find and say a little, running as a `bots` compose service.

**Architecture:** First two engine changes: dropped items decay, and a server-side `Bot` flag that shows as `[bot]` in `who`, explained by `help bots`. Then, in `bot/`: `Client.ReadChunk` (one prompt-terminated burst of output), pure parsers over chunk text, the hunting-ground and phrase tables, and `Adventurer`, a state machine (goHome → town → travel → hunt → donate, with fight/rest as procedures) driven by chunks. `cmd/watchmud-bots` supervises one adventurer per configured name and reconnects forever.

**Tech Stack:** Go standard library (`math/rand/v2`, `regexp`, `net`), testify, docker compose.

**Spec:** `docs/superpowers/specs/2026-09-30-inhabitant-bots-design.md`

## Global Constraints

- `bot/` non-test files import only the standard library.
- A bot never answers a tell except with `I'm a bot (see 'help bots') - I can't chat, sorry!`, at most once per sender per 10 minutes, and never to a sibling.
- A bot never fights in a room with a real player in it. On seeing one in a patrol room, it leaves that ground for 10 minutes.
- The hedge-witch is never prey.
- Dropped items decay after `rules.DroppedDecay` = 30 minutes. Zone-reset and wizard-loaded objects don't, unless someone drops them.
- The `Bot` flag is set only by hand (mongosh / `make bot`); nothing in the game grants it.
- At most `bot.MaxBots` = 5 bots per `watchmud-bots` process.
- New Go files go through `gofmt -w` before the gate: the plan's code blocks aren't column-aligned.
- `make check` stays green after every task; each task is committed to master on its own. Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **Plan ruling (differs from the spec):** the Hollowfields band is power **0**–5, not 1–5. Broken gear stops counting toward power (`Equipment.Power`), so a bot whose kit has all worn out is power 0; with a floor of 1 it would idle in town forever.

## Review Focus

- **A tell from someone who logs off before the bot answers.** The reply fails with "No one by that name is playing."; the bot should carry on, not stall. Task 6, `tellAnswerRe` includes that text; `TestAsk_answersTellsFirst` covers the normal path.
- **A bot's gear all breaks.** Expect power 0, still inside the band, still hunting. Task 5, `TestGrounds_bandCoversBrokenGear`.
- **A bot is killed.** It should come back from Temple Square, rest and resume, not loop or stop. Task 6, `TestFight_death`, plus `dead` routing through `goHome`.
- **A bot flees into an unknown room.** Expect it to recall and rest, not walk its patrol from the wrong place. Task 6, `TestFight_fleesWhenHurt` asserts `errLost`, which `Run` routes to `goHome`.
- **The server restarts under the bots (every deploy).** Expect `Run` to return an error and the supervisor to log in again after a jittered wait. Task 8, `superviseForever` is exercised by hand in its step 4, and in the image.

---

### Task 1: Dropped items decay

**Files:**
- Modify: `rules/power.go` (add `DroppedDecay` after `CorpseDecay`)
- Modify: `world/h_drop.go` (set `DecaysAt` after the move)
- Modify: `world/h_get.go` (clear `DecaysAt` after the move from the floor)
- Modify: `world/corpse.go` (rename `DecayCorpses`→`DecayFloors`, `decayCorpses`→`decayFloors`)
- Modify: `server/gameserver.go:117` (the caller; pulse label `"floors"`)
- Modify: `world/loot_test.go` (the two `decayCorpses` calls)
- Test: `world/h_drop_test.go`

**Interfaces:**
- Produces: `rules.DroppedDecay time.Duration`, `(*World).DecayFloors()`, `(*World).decayFloors(now time.Time)`.

- [ ] **Step 1: Write the failing tests** (append to `world/h_drop_test.go`; add `"time"` to its imports)

```go
// Dropped things don't last forever: a floor somebody keeps dropping things
// on -- the donation room, where the bots leave what they find -- would
// otherwise only ever grow.
func (s *HandleDropSuite) TestDroppedItemsDecay() {
	s.get("knife")
	before := time.Now()
	s.drop("knife")

	knives := s.w.StartRoom.Inventory.FindAll("knife")
	s.Require().Len(knives, 1)
	s.Assert().WithinDuration(before.Add(rules.DroppedDecay), knives[0].DecaysAt, time.Second)

	s.r.Sent = nil
	s.w.decayFloors(time.Now().Add(rules.DroppedDecay + time.Second))
	s.Assert().Empty(s.w.StartRoom.Inventory.FindAll("knife"), "crumbled")
	s.Assert().Equal("knife", sent[event.Decayed](s.T(), s.r, 0).Item)
}

// Carrying something is never decay: picking it up stops the clock, and
// dropping it again starts a new one.
func (s *HandleDropSuite) TestPickingUpStopsTheClock() {
	s.get("knife")
	s.drop("knife")
	s.get("knife")

	knives := s.p.Inventory().FindAll("knife")
	s.Require().Len(knives, 1)
	s.Assert().True(knives[0].DecaysAt.IsZero())
}
```

- [ ] **Step 2: Run them and watch them fail**

Run: `go test ./world -run TestHandleDropSuite`
Expected: a build failure, `undefined: rules.DroppedDecay` and `s.w.decayFloors undefined`.

- [ ] **Step 3: Implement it.** In `rules/power.go`, after `CorpseDecay`:

```go
// DroppedDecay is how long something a character drops lasts on the floor
// before it crumbles: long enough for someone to find a donation, short
// enough that a room somebody -- or a bot -- keeps dropping things in turns
// over. Zone resets and wizard loads never go through drop, so what they put
// down stays. A placeholder; LEVELS.md.
const DroppedDecay = 30 * time.Minute
```

In `world/h_drop.go`, directly after the `object.Move` error check (before `room.Send(event.Dropped{...})`):

```go
		// on the floor now, so on the clock: see rules.DroppedDecay
		objectToDrop.DecaysAt = time.Now().Add(rules.DroppedDecay)
```

(add `"time"` to its imports, and `rules` if it isn't there).

In `world/h_get.go`, directly after the `object.Move(item, room.Inventory, msg.Player.Inventory())` error check:

```go
		// carrying something is never decay; dropping it again starts afresh
		item.DecaysAt = time.Time{}
```

(add `"time"` to its imports).

In `world/corpse.go`, rename and re-comment the pair:

```go
// DecayFloors clears away everything on a floor whose time is up -- corpses
// and anything dropped -- based on time.Now().
func (w *World) DecayFloors() {
	w.decayFloors(time.Now())
}

// decayFloors removes whatever on a room's floor has a DecaysAt that has
// passed -- anything still inside goes with it -- and tells the room.
func (w *World) decayFloors(now time.Time) {
```

and change the log message inside from `"decayCorpses: removing %s from %s"` to `"decayFloors: removing %s from %s"`.

In `server/gameserver.go`, the caller becomes `recoverPulse("floors", gs.world.DecayFloors)`, and its comment's first word "on the same pulse: CorpseDecay is minutes" becomes "on the same pulse: CorpseDecay and DroppedDecay are minutes".

In `world/loot_test.go`, both `s.w.decayCorpses(` become `s.w.decayFloors(`.

- [ ] **Step 4: Run them and see them pass, then the gate**

Run: `go test ./world ./server && make check`
Expected: PASS, green.

- [ ] **Step 5: Commit**

```bash
git add rules/power.go world/ server/gameserver.go
git commit -m "dropped things crumble after half an hour

Nothing but a corpse ever left a floor, so a room somebody keeps
dropping things in -- the donation room, once bots are donating --
only grew. drop starts the clock, get stops it; zone-reset objects
never go through drop and stay. DecayCorpses is DecayFloors now.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: The Bot flag, and `[bot]` in `who`

**Files:**
- Modify: `player/player.go` (field `bot`, `IsBot`, `SetBot` beside `wizard`)
- Modify: `player/record.go` (`Bot bool` beside `Wizard` in `Record`; copied in `FromRecord` and `Record()`)
- Modify: `mongostore/document.go` (`Bot bool \`bson:"bot,omitempty"\`` beside `Wizard`, both conversions)
- Modify: `event/events.go` (`Bot bool` on `WhoEntry`)
- Modify: `world/h_who.go` (`Bot: p.IsBot()`)
- Modify: `telnet/render.go` (`whoTitle`)
- Modify: `Makefile` (a `bot` target beside `wizard`)
- Test: `player/player_test.go`, `mongostore/document_test.go`, `telnet/render_test.go`

**Interfaces:**
- Produces: `(*player.Player).IsBot() bool`, `SetBot(bool)`, `player.Record.Bot`, `event.WhoEntry.Bot`.

- [ ] **Step 1: Write the failing tests**

Append to `player/player_test.go`:

```go
// The bot flag is on the record like Wizard, for the same reason: a save
// that forgot it would unlabel every bot at the next timed save.
func (s *PlayerSuite) TestBotSurvivesTheRecord() {
	p := NewTestPlayer(uuid.New(), "wren", &Recorder{})
	s.Assert().False(p.IsBot(), "nobody starts as one")
	p.SetBot(true)

	rec := p.Record()
	s.Assert().True(rec.Bot)

	cat, err := rules.NewTestCatalog()
	s.Require().NoError(err)
	back, err := FromRecord(rec, &Recorder{}, cat, nil)
	s.Require().NoError(err)
	s.Assert().True(back.IsBot())
}
```

In `mongostore/document_test.go`, add `Bot: true,` on the line after `Wizard: true,` in `testRecord()` (the round-trip test then covers it).

In `telnet/render_test.go`, add a case to `commandCases` directly after the "who shows the role" case:

```go
	{
		// a bot is labelled where players look for who is around: the label
		// is the server's, from the record, never something a client claims
		name: "who marks bots",
		setup: func(_ *world.World, _ *player.Player, o *player.Player) {
			o.SetBot(true)
		},
		input: "who",
		want:  "-- Who Is Here --\notherdood [bot] the Human - Temple Square - Wrathrock\ntestdood the Human - Temple Square - Wrathrock\n",
	},
```

- [ ] **Step 2: Run them and watch them fail**

Run: `go test ./player ./mongostore ./telnet`
Expected: build failures, `p.IsBot undefined`, `unknown field Bot`, `o.SetBot undefined`.

- [ ] **Step 3: Implement it**

`player/player.go`, after the `wizard bool` field and its comment:

```go
	// bot marks a character a program plays, so who can say so. Nothing in
	// the game grants it: it is set by hand on the record (make bot NAME=...).
	bot bool
```

and beside `IsWizard`/`SetWizard`:

```go
func (p *Player) IsBot() bool     { return p.bot }
func (p *Player) SetBot(bot bool) { p.bot = bot }
```

`player/record.go`: add `Bot bool` to `Record` on the line after `Wizard bool` (keep gofmt alignment), `p.bot = rec.Bot` after `p.wizard = rec.Wizard`, and `Bot: p.bot,` after `Wizard: p.wizard,`.

`mongostore/document.go`: after `Wizard bool \`bson:"wizard,omitempty"\``, add `Bot bool \`bson:"bot,omitempty"\``; after `Wizard: r.Wizard,` add `Bot: r.Bot,`; after `Wizard: d.Wizard,` add `Bot: d.Bot,`.

`event/events.go`, in `WhoEntry`, after `PlayerName string`:

```go
	// Bot is the record's flag: a program plays this character.
	Bot bool
```

`world/h_who.go`, in the `event.WhoEntry{...}` literal, after `PlayerName: p.Name(),`: `Bot: p.IsBot(),`.

`telnet/render.go`, `whoTitle` becomes:

```go
func whoTitle(p event.WhoEntry) string {
	name := p.PlayerName
	if p.Bot {
		name += " [bot]"
	}
	var parts []string
	if p.Lineage != "" {
		parts = append(parts, p.Lineage)
	}
	if p.Role != "" {
		parts = append(parts, p.Role)
	}
	if len(parts) == 0 {
		return name
	}
	return name + " the " + strings.Join(parts, " ")
}
```

`Makefile`, after the `wizard:` target (same shape; the `##` comment line follows the file's convention):

```make
## bot: label a character as a bot in `who` (while it's logged out); UNSET=1 to take it off
.PHONY: bot
bot:
	@test -n "$(NAME)" || (echo "usage: make bot NAME=<character> [UNSET=1]" && exit 1)
	docker compose exec mongo mongosh watchmud --quiet --eval \
		'const n = "$(NAME)", name = n[0].toUpperCase() + n.slice(1).toLowerCase(); const r = db.players.updateOne({name}, {$$set: {bot: $(if $(UNSET),false,true)}}); print(r.matchedCount ? name + ": bot=$(if $(UNSET),false,true)" : "no character named " + name)'
```

- [ ] **Step 4: Run them and see them pass, then the gate**

Run: `go test ./player ./mongostore ./telnet ./world && make check`
Expected: PASS, green.

- [ ] **Step 5: Commit**

```bash
git add player/ mongostore/ event/events.go world/h_who.go telnet/render.go telnet/render_test.go Makefile
git commit -m "a bot is labelled in who, from a flag on its record

Bot beside Wizard: on the record and the mongo document, set by hand
(make bot NAME=...), never by the game or the client. who shows
'Wren [bot]'.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: `help bots`

**Files:**
- Modify: `telnet/help.go` (topics, `helpFor`, the "More" section)
- Modify: `telnet/conn.go:759-764` (the command loop answers `helpFor`)
- Test: `telnet/help_test.go`

**Interfaces:**
- Produces: `helpFor(line string) (string, bool)`; `isHelp` keeps its signature (the name prompt uses it).

- [ ] **Step 1: Write the failing tests** (append to `telnet/help_test.go`)

```go
// help takes a topic, for what isn't a command -- the first is who the bots
// are -- and the command list says which topics there are.
func TestHelp_topics(t *testing.T) {
	text, ok := helpFor("help bots")
	require.True(t, ok)
	assert.Contains(t, text, "programs, not people")

	again, ok := helpFor("  HELP   Bots ")
	require.True(t, ok)
	assert.Equal(t, text, again)

	text, ok = helpFor("help dragons")
	require.True(t, ok, "an unknown topic is still help, not a command")
	assert.Equal(t, "There's no help on that. Type 'help' for the commands.\n", text)

	_, ok = helpFor("helpful")
	assert.False(t, ok)

	assert.Contains(t, helpText, "help bots")
	for name, topic := range helpTopics {
		for _, l := range strings.Split(strings.TrimRight(topic.text, "\n"), "\n") {
			assert.LessOrEqual(t, utf8.RuneCountInString(l), 79, "help %s: %q", name, l)
		}
	}
}

func TestHelp_topicInGame(t *testing.T) {
	gs := &fakeServer{passwords: map[string]string{"Bob": "sekrit99"}}
	s := startSession(t, gs)
	s.answer("known? ", "Bob")
	s.answer("Password: ", "sekrit99")
	s.answer(echoOn, "help bots")
	s.waitFor("programs, not people")
	assert.Len(t, gs.commands(), 1, "only the login reached the server")
}
```

- [ ] **Step 2: Run them and watch them fail**

Run: `go test ./telnet -run 'TestHelp_'`
Expected: a build failure, `undefined: helpFor`, `undefined: helpTopics`.

- [ ] **Step 3: Implement it.** In `telnet/help.go`, add `"maps"` and `"slices"` to the imports, and after `helpSections`:

```go
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
```

In `helpText`'s builder, after the sections loop and before `return b.String()`:

```go
	b.WriteString("\nMore\n")
	for _, name := range slices.Sorted(maps.Keys(helpTopics)) {
		fmt.Fprintf(&b, "  %-26s %s\n", "help "+name, helpTopics[name].what)
	}
```

Replace `isHelp` with:

```go
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
```

In `telnet/conn.go`'s command loop, replace

```go
		if isHelp(line) {
			// the world never hears it, so the prompt has to come from here
			c.Send(helpText)
```

with

```go
		if text, ok := helpFor(line); ok {
			// the world never hears it, so the prompt has to come from here
			c.Send(text)
```

- [ ] **Step 4: Run them and see them pass, then the gate**

Run: `go test ./telnet && make check`
Expected: PASS (including `TestHelp_fitsOneScreen`, now 36 lines), green.

- [ ] **Step 5: Commit**

```bash
git add telnet/help.go telnet/help_test.go telnet/conn.go
git commit -m "help takes a topic, and the first is who the bots are

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: `ReadChunk`, and reading a chunk

**Files:**
- Modify: `bot/client.go` (`newClient`, `Chunk`, `ReadChunk`)
- Create: `bot/perceive.go`
- Test: `bot/client_test.go`, `bot/perceive_test.go`

**Interfaces:**
- Produces:
  - `func newClient(nc net.Conn) *Client` (`Dial` uses it; tests hand it one end of a `net.Pipe`)
  - `type Chunk struct{ Text string; Health, MaxHealth int }`
  - `func (c *Client) ReadChunk(timeout time.Duration) (Chunk, error)`. On timeout it returns `Chunk{}` and a nil error (`MaxHealth == 0` means no prompt arrived). A closed connection is an error.
  - In `perceive.go`: `tellers(text string) []string`, `attacked(text string) bool`, `blows(text string) bool`, `deaths(text string) []string`, `playersHere(text, self string, siblings []string) []string`, `looted(text string) int`, constants `youDied`, `youFled`, and regexps `powerRe`, `considerRe`.

- [ ] **Step 1: Write the failing tests.** Append to `bot/client_test.go`:

```go
// A chunk is everything up to a prompt, and the prompt is where the health is.
func TestReadChunk_splitsOnPrompts(t *testing.T) {
	c, nc := connect(t)
	_, _ = io.WriteString(nc, "The Millpond\r\n A pond.\r\n<97/100hp> angry goose hits you for 2 damage.\r\n<95/100hp> ")

	ch, err := c.ReadChunk(time.Second)
	require.NoError(t, err)
	assert.Equal(t, Chunk{Text: "The Millpond\n A pond.\n", Health: 97, MaxHealth: 100}, ch)

	ch, err = c.ReadChunk(time.Second)
	require.NoError(t, err)
	assert.Equal(t, Chunk{Text: "angry goose hits you for 2 damage.\n", Health: 95, MaxHealth: 100}, ch)
}

// Nothing happening is not a failure: a resting bot waits on a quiet world.
func TestReadChunk_quietIsNotAnError(t *testing.T) {
	c, nc := connect(t)
	_, _ = io.WriteString(nc, "half a line with no prompt")
	ch, err := c.ReadChunk(50 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, Chunk{}, ch)

	_, _ = io.WriteString(nc, "\r\n<100/100hp> ")
	ch, err = c.ReadChunk(time.Second)
	require.NoError(t, err)
	assert.Equal(t, "half a line with no prompt\n", ch.Text, "the partial text waited for its prompt")
}

func TestReadChunk_closedIsAnError(t *testing.T) {
	c, nc := connect(t)
	require.NoError(t, nc.Close())
	_, err := c.ReadChunk(time.Second)
	assert.Error(t, err)
}
```

Create `bot/perceive_test.go`:

```go
package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTellers(t *testing.T) {
	text := "Bob tells you, \"hi\".\nangry goose hits you for 2 damage.\nAnn tells you, \"hello\".\n"
	assert.Equal(t, []string{"Bob", "Ann"}, tellers(text))
	assert.Empty(t, tellers("You say, \"Bob tells you, hi\".\n"), "only at the start of a line")
}

func TestAttackedAndBlows(t *testing.T) {
	assert.True(t, attacked("angry goose hits you for 2 damage.\n"))
	assert.True(t, attacked("field rat misses you.\n"))
	assert.False(t, attacked("You hit field rat for 3 damage.\n"))
	assert.False(t, attacked("Pim hits field rat.\n"), "someone else's fight")

	assert.True(t, blows("You miss field rat.\n"))
	assert.True(t, blows("field rat misses you.\n"))
	assert.False(t, blows("Pim hits field rat.\n"))
}

func TestDeaths(t *testing.T) {
	assert.Equal(t, []string{"angry goose", "field rat"}, deaths("angry goose is dead!\nYou hit x.\nfield rat is dead!\n"))
}

// Fellow bots and yourself aren't players to make way for.
func TestPlayersHere(t *testing.T) {
	room := "The Millpond\n A pond.\n[ Exits: e s ]\nAn angry goose lowers its neck and hisses at you.\nWren is here.\nPim is here.\nBob is here.\n"
	assert.Equal(t, []string{"Bob"}, playersHere(room, "wren", []string{"Pim", "Odo"}))
	assert.Empty(t, playersHere("The corpse of field rat is lying here.\n", "Wren", nil))
}

func TestLooted(t *testing.T) {
	assert.Equal(t, 2, looted("You get a scrap of rat pelt from the corpse of field rat.\nYou get a knotted cudgel from the corpse of bandit.\n"))
	assert.Equal(t, 0, looted("There's nothing in there.\n"))
}

func TestPowerAndConsider(t *testing.T) {
	m := powerRe.FindStringSubmatch("You are using (power 3):\nwield\ta dagger\n")
	assert.Equal(t, "3", m[1])
	m = considerRe.FindStringSubmatch("Giant beetle looks like a fair fight. (power 2; you are 1)\n")
	assert.Equal(t, []string{"(power 2; you are 1)", "2", "1"}, m)
}
```

- [ ] **Step 2: Run them and watch them fail**

Run: `go test ./bot`
Expected: a build failure, `undefined: Chunk`, `undefined: tellers`.

- [ ] **Step 3: Implement.** In `bot/client.go`, add `"strconv"` to the imports, and make `Dial` construct through a new function:

```go
		if err == nil {
			return newClient(nc), nil
		}
```

```go
// newClient starts reading nc. Dial is the usual way in; tests hand it one
// end of a net.Pipe.
func newClient(nc net.Conn) *Client {
	c := &Client{nc: nc, changed: make(chan struct{})}
	go c.read()
	return c
}
```

Then, after `ExpectClosed`:

```go
// Chunk is what the server said up to a prompt: a room, a round of a fight, a
// tell. Every burst of output ends in a prompt, so a chunk is the bot's unit
// of perception, and the prompt is where it reads its health.
type Chunk struct {
	Text      string
	Health    int
	MaxHealth int // zero: no prompt arrived before the timeout
}

var promptRe = regexp.MustCompile(`<(\d+)/(\d+)hp> `)

// ReadChunk consumes through the next prompt and returns what came before it.
// With no prompt within timeout it returns an empty Chunk and leaves anything
// partial for the next call: a quiet world isn't an error. A closed
// connection is.
func (c *Client) ReadChunk(timeout time.Duration) (Chunk, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		c.mu.Lock()
		if loc := promptRe.FindSubmatchIndex(c.unread); loc != nil {
			ch := Chunk{Text: string(c.unread[:loc[0]])}
			ch.Health, _ = strconv.Atoi(string(c.unread[loc[2]:loc[3]]))
			ch.MaxHealth, _ = strconv.Atoi(string(c.unread[loc[4]:loc[5]]))
			c.unread = c.unread[loc[1]:]
			c.mu.Unlock()
			return ch, nil
		}
		closed, changed := c.closed, c.changed
		c.mu.Unlock()
		if closed {
			return Chunk{}, fmt.Errorf("connection closed; last received:\n%s", c.tail())
		}
		select {
		case <-changed:
		case <-deadline.C:
			return Chunk{}, nil
		}
	}
}
```

Create `bot/perceive.go`:

```go
package bot

import (
	"regexp"
	"strings"
)

// What a bot reads out of a chunk. Every pattern is text telnet/render.go
// writes; when the rendering changes, these are what should notice.

var (
	tellRe     = regexp.MustCompile(`(?m)^([A-Z][a-z]+) tells you, "`)
	attackedRe = regexp.MustCompile(`(?m)^.+ (?:hits you for \d+ damage|misses you)\.$`)
	blowRe     = regexp.MustCompile(`(?m)^(?:.+ (?:hits you for \d+ damage|misses you)|You (?:hit|miss) .+)\.$`)
	deathRe    = regexp.MustCompile(`(?m)^(.+) is dead!$`)
	hereRe     = regexp.MustCompile(`(?m)^([A-Z][a-z]+) is here\.$`)
	gotRe      = regexp.MustCompile(`(?m)^You get .+ from .+\.$`)
	powerRe    = regexp.MustCompile(`You are using \(power (\d+)\)`)
	considerRe = regexp.MustCompile(`\(power (\d+); you are (\d+)\)`)
)

const (
	youDied = "You are dead!"
	youFled = "You flee head over heels."
)

// tellers is who told this bot something, in order.
func tellers(text string) []string {
	var out []string
	for _, m := range tellRe.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
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

// looted is how many things one "get all from corpse" took.
func looted(text string) int { return len(gotRe.FindAllString(text, -1)) }
```

- [ ] **Step 4: Run them and see them pass, then the gate**

Run: `go test -race ./bot && make check`
Expected: PASS, green.

- [ ] **Step 5: Commit**

```bash
git add bot/
git commit -m "bot: read the world a prompt at a time, and make sense of it

ReadChunk returns everything up to the next prompt, and the health in
it; quiet is not an error. perceive.go reads tells, blows, deaths,
players and loot out of a chunk.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Hunting grounds and phrases, walked against the real content

**Files:**
- Create: `bot/grounds.go`, `bot/talk.go`
- Create: `bot/game_test.go` (package `bot`: `startGame`, `dial`, `createCharacter`, `loginAs`, moved out of `smoke_test.go`)
- Modify: `bot/smoke_test.go` (becomes package `bot`; its helpers move to `game_test.go`)
- Test: `bot/grounds_test.go`

**Interfaces:**
- Produces:
  - `type ground struct{ name string; minPower, maxPower int; route, patrol []step; prey []prey; loot []string }`
  - `type step struct{ dir, room string }`
  - `type prey struct{ keyword, name, seen string }`
  - `var grounds []ground`
  - `func (g *ground) preyIn(room string) []prey`
  - `func (g *ground) isPrey(name string) bool`
  - `type moment int` with `momentKill`, `momentTown`, `momentRest`, `momentDonate`
  - `var phrases map[string]map[moment][]string`, keyed by lowercase bot name
  - Test helpers: `startGame(t, tick time.Duration) string`, `dial(t, addr) *Client`, `createCharacter(t, addr, name, password)`, `loginAs(t, addr, name, password) *Client`

- [ ] **Step 1: Move the in-process helpers into package `bot`.** Create `bot/game_test.go`:

```go
package bot

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/dice"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/memstore"
	"github.com/watchmud/watchmud/server"
	"github.com/watchmud/watchmud/telnet"
	"github.com/watchmud/watchmud/world"
)

// startGame runs the real content -- not testcontent: a content change that
// breaks a bot is exactly what these tests are for -- on 127.0.0.1:0,
// ticking every tick. 10ms is a hundred times the real pace; an hour is a
// world where no pulse ever comes, so nothing wanders or picks a fight.
func startGame(t *testing.T, tick time.Duration) string {
	t.Helper()
	content, err := loader.LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	store := memstore.New()
	w, err := world.New(content, store, dice.New([32]byte{}))
	require.NoError(t, err)
	gs := server.New(w, content.Catalog, store)
	gs.SetTickInterval(tick)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{}, 2)
	go func() { _ = telnet.Serve(ctx, ln, gs, content.Catalog); done <- struct{}{} }()
	go func() { _ = gs.Run(ctx); done <- struct{}{} }()
	t.Cleanup(func() { cancel(); <-done; <-done })
	return ln.Addr().String()
}

func dial(t *testing.T, addr string) *Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// createCharacter is the conversation a person has once, by hand, on
// production. It lives in a test, so no binary can reach it.
func createCharacter(t *testing.T, addr, name, password string) {
	t.Helper()
	c := dial(t, addr)
	steps := []struct{ want, send string }{
		{`By what name do you wish to be known\? `, name},
		{`Create them\? \(yn\) `, "y"},
		{`Which lineage\? `, "1"},
		{`Choose a password: `, password},
		{`Again: `, password},
		{`\[ Exits: `, "quit"},
	}
	for _, s := range steps {
		_, err := c.Expect(s.want, 10*time.Second)
		require.NoError(t, err, c.Transcript())
		require.NoError(t, c.Send(s.send))
	}
	require.NoError(t, c.ExpectClosed(10*time.Second), c.Transcript())
}

// loginAs is a player at the keyboard, for a test that needs one in the world.
func loginAs(t *testing.T, addr, name, password string) *Client {
	t.Helper()
	c := dial(t, addr)
	require.NoError(t, login(c, Config{Name: name, Password: password}), c.Transcript())
	return c
}
```

Replace `bot/smoke_test.go` with:

```go
package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmoke_againstTheRealWorld(t *testing.T) {
	addr := startGame(t, 10*time.Millisecond)
	createCharacter(t, addr, "Tester", "correcthorse")

	c := dial(t, addr)
	var log strings.Builder
	res, err := Smoke(c, Config{Name: "Tester", Password: "correcthorse"}, &log)
	t.Log("\n" + log.String())
	require.NoError(t, err, "%s\n--- transcript ---\n%s", log.String(), c.Transcript())
	assert.Empty(t, res.Notes, "a fresh world has geese, so the fight must have been tested")
}
```

Run: `go test -race ./bot`
Expected: PASS. This step is a refactor, so it must stay green.

- [ ] **Step 2: Write the failing ground tests** in `bot/grounds_test.go`:

```go
package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every ground's route and patrol, walked against the real content in a world
// with no pulses, so nothing picks a fight on the way round. A renamed room
// or a moved exit fails here, not in production.
func TestGrounds_walk(t *testing.T) {
	addr := startGame(t, time.Hour)
	createCharacter(t, addr, "Walker", "correcthorse")
	for _, g := range grounds {
		t.Run(g.name, func(t *testing.T) {
			require.NotEmpty(t, g.route)
			require.NotEmpty(t, g.patrol)
			assert.Equal(t, g.route[len(g.route)-1].room, g.patrol[len(g.patrol)-1].room,
				"the patrol is a loop from where the route ends")

			c := loginAs(t, addr, "Walker", "correcthorse")
			require.NoError(t, recall(c))
			for _, s := range append(append([]step{}, g.route...), g.patrol...) {
				require.NoError(t, c.Send(s.dir))
				_, err := room(c, s.room)
				require.NoError(t, err, "going %s to %s", s.dir, s.room)
			}
			require.NoError(t, c.Send("quit"))
			require.NoError(t, c.ExpectClosed(5*time.Second))
		})
	}
}

// Broken gear stops counting toward power, so a bot in worn-out kit is power
// 0. It should still have somewhere to hunt, not idle in town forever.
func TestGrounds_bandCoversBrokenGear(t *testing.T) {
	found := false
	for _, g := range grounds {
		if g.minPower == 0 {
			found = true
		}
	}
	assert.True(t, found)
}

func TestGround_preyIn(t *testing.T) {
	g := &grounds[0]
	room := "The Red Barn\n A barn.\n[ Exits: n e ]\nA fat field rat noses through the straw.\nA fat field rat noses through the straw.\n"
	got := g.preyIn(room)
	require.Len(t, got, 1, "one of each kind; the second rat waits for the next lap")
	assert.Equal(t, "rat", got[0].keyword)
	assert.True(t, g.isPrey("field rat"))
	assert.False(t, g.isPrey("hedge-witch"), "her ring and censer are for players")
}

// A bot with no phrases is quiet, not broken; one with phrases has something
// for every moment.
func TestPhrases(t *testing.T) {
	for name, byMoment := range phrases {
		for _, m := range []moment{momentKill, momentTown, momentRest, momentDonate} {
			assert.NotEmpty(t, byMoment[m], "%s has nothing to say at moment %d", name, m)
		}
	}
	assert.Nil(t, phrases["nobody"][momentKill])
}
```

Run: `go test ./bot -run 'TestGround|TestPhrases'`
Expected: a build failure, `undefined: grounds`, `undefined: phrases`.

- [ ] **Step 3: Write `bot/grounds.go`**

```go
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
```

- [ ] **Step 4: Write `bot/talk.go`**

```go
package bot

// moment is when a bot might say something.
type moment int

const (
	momentKill moment = iota
	momentTown
	momentRest
	momentDonate
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
	},
	"pim": {
		momentKill:   {"Ha!", "Too slow!"},
		momentTown:   {"Home again, home again.", "Who's buying?"},
		momentRest:   {"Just catching my breath...", "That goose bites harder than it looks."},
		momentDonate: {"Free stuff in the donation room, folks.", "Help yourselves."},
	},
	"odo": {
		momentKill:   {"Rest easy, little beast.", "Sorry about that."},
		momentTown:   {"The fields are quiet today.", "Good to see the square busy."},
		momentRest:   {"Hm. That stung.", "Sitting down for a bit."},
		momentDonate: {"Leaving these for whoever needs them.", "For the new ones."},
	},
}
```

- [ ] **Step 5: Run them and see them pass, then the gate**

Run: `go test -race ./bot && make check`
Expected: PASS, green. `TestGrounds_walk` walks 13 rooms against the real content.

- [ ] **Step 6: Commit**

```bash
git add bot/
git commit -m "bot: the Hollowfields as a hunting ground, and what bots say

Route, patrol loop, prey and loot as a Go table, walked against the
real content in a world with no pulses. Never the hedge-witch. The
in-process test helpers move into package bot so both suites share
them.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: `Adventurer`

**Files:**
- Create: `bot/adventurer.go`
- Test: `bot/adventurer_test.go` (package `bot`, against a `net.Pipe`)

**Interfaces:**
- Consumes: `newClient`, `Client.ReadChunk`/`Send`/`ExpectClosed`/`Expect` (Tasks 2 and 4 of the smoke plan, and Task 4 here); `login`, `Config` (smoke.go); the perceive helpers (Task 4); `grounds`, `step`, `prey`, `phrases`, `moment*` (Task 5).
- Produces:
  - `const MaxBots = 5`
  - `type Pace struct{ Think, Walk [2]time.Duration; Quiet, Poll, Answer, Idle time.Duration }` and `var HumanPace Pace`
  - `type AdventurerConfig struct{ Name, Password string; Siblings []string; Seed uint64; Pace Pace; Log func(format string, args ...any) }`
  - `type Stats struct{ Kills, Looted, Donations, TellsAnswered, Avoided, Deaths int }`
  - `func NewAdventurer(cfg AdventurerConfig) *Adventurer`
  - `func (a *Adventurer) Run(ctx context.Context, addr string) error`, which returns nil after quitting cleanly on ctx done
  - `func (a *Adventurer) Stats() Stats`, safe from any goroutine

- [ ] **Step 1: Write the failing tests** in `bot/adventurer_test.go`:

```go
package bot

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pipeGame is the server end of a net.Pipe: the test says what the game says
// and hears each line the bot types.
type pipeGame struct {
	t     *testing.T
	nc    net.Conn
	lines chan string
}

var quickPace = Pace{Quiet: 100 * time.Millisecond, Poll: time.Millisecond, Answer: time.Second, Idle: 10 * time.Millisecond}

// adventurer is a quiet bot (no phrases for "Testbot") on a pipe.
func adventurer(t *testing.T) (*Adventurer, *pipeGame) {
	t.Helper()
	server, client := net.Pipe()
	a := NewAdventurer(AdventurerConfig{Name: "Testbot", Password: "x", Siblings: []string{"Pim"}, Seed: 1, Pace: quickPace})
	a.c = newClient(client)
	g := &pipeGame{t: t, nc: server, lines: make(chan string, 64)}
	go func() {
		r := bufio.NewReader(server)
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				close(g.lines)
				return
			}
			g.lines <- strings.TrimRight(l, "\r\n")
		}
	}()
	t.Cleanup(func() { _ = a.c.Close(); _ = server.Close() })
	return a, g
}

func (g *pipeGame) say(s string) {
	g.t.Helper()
	_, err := g.nc.Write([]byte(s))
	require.NoError(g.t, err)
}

func (g *pipeGame) heard() string {
	g.t.Helper()
	select {
	case l := <-g.lines:
		return l
	case <-time.After(2 * time.Second):
		g.t.Fatal("the bot said nothing")
		return ""
	}
}

// One honest answer per person per ten minutes, and never to a fellow bot --
// two bots answering each other would never stop.
func TestTold_oncePerSenderAndNeverASibling(t *testing.T) {
	a, _ := adventurer(t)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }

	a.told("Bob")
	a.told("Bob")
	a.told("Pim")
	a.told("testbot")
	assert.Equal(t, []string{"Bob"}, a.pendingTells)

	a.pendingTells = nil
	now = now.Add(9 * time.Minute)
	a.told("Bob")
	assert.Empty(t, a.pendingTells, "still inside ten minutes")
	now = now.Add(2 * time.Minute)
	a.told("Bob")
	assert.Equal(t, []string{"Bob"}, a.pendingTells)
}

func TestAsk_answersTellsFirst(t *testing.T) {
	a, g := adventurer(t)
	a.pendingTells = []string{"Bob"}
	done := make(chan error, 1)
	go func() {
		_, _, err := a.ask("look", roomRe("Temple Square"))
		done <- err
	}()

	assert.Equal(t, "tell Bob I'm a bot (see 'help bots') - I can't chat, sorry!", g.heard())
	g.say("Ok.\r\n<100/100hp> ")
	assert.Equal(t, "look", g.heard())
	g.say("Temple Square\r\n A square.\r\n[ Exits: n e s ]\r\n<100/100hp> ")
	require.NoError(t, <-done)
	assert.Equal(t, 1, a.Stats().TellsAnswered)
}

func TestFight_endsQuietAndLoots(t *testing.T) {
	a, g := adventurer(t)
	a.ground = &grounds[0]
	done := make(chan error, 1)
	go func() { done <- a.fight(context.Background()) }()

	g.say("You hit angry goose for 3 damage.\r\nangry goose is dead!\r\n<95/100hp> ")
	assert.Equal(t, "get all from corpse", g.heard(), "after Quiet goes by without a blow")
	g.say("You get a long goose feather from the corpse of angry goose.\r\n<95/100hp> ")
	require.NoError(t, <-done)
	assert.Equal(t, 1, a.Stats().Kills)
	assert.Equal(t, 1, a.Stats().Looted)
	assert.Equal(t, 1, a.carrying)
	assert.False(t, a.attacked)
}

func TestFight_fleesWhenHurt(t *testing.T) {
	a, g := adventurer(t)
	a.ground = &grounds[0]
	a.attacked = true
	done := make(chan error, 1)
	go func() { done <- a.fight(context.Background()) }()

	g.say("wild dog hits you for 60 damage.\r\n<20/100hp> ")
	assert.Equal(t, "flee", g.heard())
	g.say("You flee head over heels.\r\nFarm Lane\r\n A lane.\r\n[ Exits: n s e w ]\r\n<20/100hp> ")
	err := <-done
	assert.ErrorIs(t, err, errLost, "it's somewhere it didn't choose: recall")
}

func TestFight_death(t *testing.T) {
	a, g := adventurer(t)
	a.ground = &grounds[0]
	a.attacked = true
	done := make(chan error, 1)
	go func() { done <- a.fight(context.Background()) }()

	g.say("wild dog hits you for 9 damage.\r\nYou are dead!\r\nTemple Square\r\n A square.\r\n[ Exits: n e s ]\r\n<1/100hp> ")
	require.NoError(t, <-done)
	assert.True(t, a.died)
	assert.Equal(t, 1, a.health)
}

// A real player in a patrol room has the ground to themselves for a while.
func TestHunt_leavesAGroundToAPlayer(t *testing.T) {
	a, g := adventurer(t)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	a.ground = &grounds[0]
	done := make(chan error, 1)
	go func() {
		next, err := a.hunt(context.Background())
		assert.NotNil(t, next)
		done <- err
	}()

	assert.Equal(t, "west", g.heard())
	g.say("The Millpond\r\n A pond.\r\n[ Exits: e s ]\r\nAn angry goose lowers its neck and hisses at you.\r\nPim is here.\r\nBob is here.\r\n<100/100hp> ")
	require.NoError(t, <-done)
	assert.Equal(t, 1, a.Stats().Avoided)
	assert.Nil(t, a.pickGround(1), "avoiding the only ground")
	now = now.Add(avoidFor + time.Minute)
	assert.Equal(t, &grounds[0], a.pickGround(1))
}

// Resting reads health off prompts it asks for with a bare Enter, since regen
// on its own prints nothing.
func TestRest_pollsUntilRested(t *testing.T) {
	a, g := adventurer(t)
	a.health, a.maxHealth = 40, 100
	done := make(chan error, 1)
	go func() { done <- a.rest(context.Background()) }()

	assert.Equal(t, "", g.heard())
	g.say("<60/100hp> ")
	assert.Equal(t, "", g.heard())
	g.say("<95/100hp> ")
	require.NoError(t, <-done)
	assert.Equal(t, 95, a.health)
}
```

- [ ] **Step 2: Run them and watch them fail**

Run: `go test ./bot -run 'TestTold|TestAsk|TestFight|TestHunt|TestRest'`
Expected: a build failure, `undefined: Pace`, `undefined: NewAdventurer`.

- [ ] **Step 3: Write `bot/adventurer.go`**

```go
package bot

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// An adventurer's rules of thumb. Constants rather than config: they are what
// makes a bot polite, and a bot shouldn't be one flag away from rude.
const (
	// MaxBots: the server allows 5 connections per address, and every bot run
	// by one watchmud-bots shares its address.
	MaxBots = 5

	donateAfter = 5                // things looted before a trip to the donation room
	avoidFor    = 10 * time.Minute // a ground a player is using is theirs this long
	tellEvery   = 10 * time.Minute // one honest answer per person this often
	talkEvery   = 10 * time.Minute // at most one remark this often
	talkOneIn   = 5                // and on only one moment in this many
	restBelow   = 50               // percent health: stop and rest
	restUntil   = 90               // percent health: rested
	fleeBelow   = 25               // percent health, mid-fight: run
	patrolLaps  = 3                // laps of a ground before heading back to town

	botReply = "I'm a bot (see 'help bots') - I can't chat, sorry!"
)

// Pace is how fast a bot plays. HumanPace is the real one; tests go faster.
type Pace struct {
	Think  [2]time.Duration // before each decision, uniformly in [min, max)
	Walk   [2]time.Duration // before each step
	Quiet  time.Duration    // a fight is over after this long with no blows
	Poll   time.Duration    // how often a resting bot looks at its health
	Answer time.Duration    // how long the server has to reply
	Idle   time.Duration    // how long a bot with nowhere to hunt waits in town
}

// HumanPace is a person at a keyboard.
var HumanPace = Pace{
	Think:  [2]time.Duration{2 * time.Second, 6 * time.Second},
	Walk:   [2]time.Duration{time.Second, 3 * time.Second},
	Quiet:  4 * time.Second,
	Poll:   5 * time.Second,
	Answer: 10 * time.Second,
	Idle:   3 * time.Minute,
}

// AdventurerConfig is one bot. Siblings are the other bots run beside it:
// never players to make way for, never answered.
type AdventurerConfig struct {
	Name, Password string
	Siblings       []string
	Seed           uint64
	Pace           Pace
	Log            func(format string, args ...any) // nil is silent
}

// Stats is what an adventurer has done since it started.
type Stats struct {
	Kills, Looted, Donations, TellsAnswered, Avoided, Deaths int
}

// Adventurer is a bot that plays like a player: out to a hunting ground, fights
// what it can, loots its own kills, rests when hurt, gives what it finds to the
// donation room. Everything but Stats belongs to the goroutine running Run.
type Adventurer struct {
	cfg AdventurerConfig
	rng *rand.Rand
	now func() time.Time // for what it remembers: tells, avoided grounds, remarks
	c   *Client

	health, maxHealth int
	attacked, died    bool
	carrying          int // looted since the last donation
	ground            *ground
	avoiding          map[string]time.Time // ground name -> until
	toldAt            map[string]time.Time
	pendingTells      []string
	saidAt            time.Time

	mu    sync.Mutex
	stats Stats
}

func NewAdventurer(cfg AdventurerConfig) *Adventurer {
	return &Adventurer{
		cfg:      cfg,
		rng:      rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9e3779b97f4a7c15)),
		now:      time.Now,
		avoiding: map[string]time.Time{},
		toldAt:   map[string]time.Time{},
	}
}

// Stats is safe to call while Run runs.
func (a *Adventurer) Stats() Stats {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stats
}

func (a *Adventurer) count(f func(*Stats)) {
	a.mu.Lock()
	f(&a.stats)
	a.mu.Unlock()
}

func (a *Adventurer) log(format string, args ...any) {
	if a.cfg.Log != nil {
		a.cfg.Log(a.cfg.Name+": "+format, args...)
	}
}

// Not the connection's fault, so not Run's to return: each sends the
// adventurer somewhere to start over.
var (
	errLost = errors.New("lost")                // an answer it didn't expect: recall
	errDied = errors.New("died")                // wakes in Temple Square
	errBusy = errors.New("in a fight it didn't start") // fight, then recall
)

var (
	okRe         = regexp.MustCompile(`(?m)^Ok\.$`)
	tellAnswerRe = regexp.MustCompile(`(?m)^Ok\.$|No one by that name is playing\.`)
	anyRe        = regexp.MustCompile(``)
	busyText     = "You're too busy fighting!"
	killRe       = regexp.MustCompile(`(?m)^Ok\.$|You don't see that here\.|You're already fighting!`)
	considerOrGoneRe = regexp.MustCompile(`\(power (\d+); you are (\d+)\)|You don't see that here\.`)
	lootRe       = regexp.MustCompile(`You get |There's nothing in there\.|You don't see that here\.`)
	dropRe       = regexp.MustCompile(`Dropped\.|You aren't carrying that\.`)
	saidRe       = regexp.MustCompile(`You say, "`)
)

// roomRe matches a room description by its name, at the start of a line.
func roomRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `$`)
}

type stateFn func(ctx context.Context) (stateFn, error)

// Run connects, logs in and adventures until ctx is done -- then it quits
// cleanly and returns nil -- or the connection fails, and it returns why, for
// whoever runs it to log it and try again later.
func (a *Adventurer) Run(ctx context.Context, addr string) error {
	dctx, cancel := context.WithTimeout(ctx, time.Minute)
	c, err := Dial(dctx, addr)
	cancel()
	if err != nil {
		return err
	}
	defer c.Close()
	a.c = c
	if _, err := c.Expect(`Welcome to WatchMUD`, a.cfg.Pace.Answer); err != nil {
		return err
	}
	if err := login(c, Config{Name: a.cfg.Name, Password: a.cfg.Password}); err != nil {
		return err
	}
	a.log("logged in")

	state := stateFn(a.goHome)
	for {
		next, err := state(ctx)
		switch {
		case ctx.Err() != nil:
			return a.quit()
		case errors.Is(err, errDied):
			next = a.dead
		case errors.Is(err, errBusy):
			next = a.fightThen(a.goHome)
		case errors.Is(err, errLost):
			a.log("%v; recalling", err)
			next = a.goHome
		case err != nil:
			return err
		}
		state = next
	}
}

func (a *Adventurer) quit() error {
	if err := a.c.Send("quit"); err != nil {
		return nil // already gone
	}
	_ = a.c.ExpectClosed(a.cfg.Pace.Answer)
	a.log("quit")
	return nil
}

// ---- states -----------------------------------------------------------------

// goHome recalls to Temple Square.
func (a *Adventurer) goHome(ctx context.Context) (stateFn, error) {
	if a.attacked {
		return a.fightThen(a.goHome), nil
	}
	if err := a.pause(ctx, a.cfg.Pace.Think); err != nil {
		return nil, err
	}
	if _, _, err := a.ask("recall", roomRe("Temple Square")); err != nil {
		return nil, err
	}
	a.say(momentTown)
	return a.town, nil
}

// town rests if it must, then picks a ground its power suits.
func (a *Adventurer) town(ctx context.Context) (stateFn, error) {
	_, m, err := a.ask("equipment", powerRe)
	if err != nil {
		return nil, err
	}
	if a.needsRest() {
		if err := a.rest(ctx); err != nil {
			return nil, err
		}
		if a.died {
			return a.dead, nil
		}
	}
	power, _ := strconv.Atoi(m[1])
	a.ground = a.pickGround(power)
	if a.ground == nil {
		a.log("nowhere to hunt at power %d; waiting in town", power)
		return a.waitThen(a.cfg.Pace.Idle, a.town), nil
	}
	a.log("off to %s at power %d", a.ground.name, power)
	return a.travel, nil
}

func (a *Adventurer) travel(ctx context.Context) (stateFn, error) {
	for _, s := range a.ground.route {
		if _, err := a.walk(ctx, s); err != nil {
			return nil, err
		}
	}
	a.log("hunting %s", a.ground.name)
	return a.hunt, nil
}

// hunt walks the patrol, fighting what's fair, leaving if a player turns up.
func (a *Adventurer) hunt(ctx context.Context) (stateFn, error) {
	g := a.ground
	for range patrolLaps {
		for _, s := range g.patrol {
			room, err := a.walk(ctx, s)
			if err != nil {
				return nil, err
			}
			if who := playersHere(room, a.cfg.Name, a.cfg.Siblings); len(who) > 0 {
				a.avoiding[g.name] = a.now().Add(avoidFor)
				a.count(func(st *Stats) { st.Avoided++ })
				a.log("%s is in %s; leaving %s to them", who[0], s.room, g.name)
				return a.goHome, nil
			}
			for _, p := range g.preyIn(room) {
				if err := a.engage(ctx, p); err != nil {
					return nil, err
				}
				if a.died {
					return a.dead, nil
				}
			}
			if a.attacked {
				if err := a.fight(ctx); err != nil {
					return nil, err
				}
				if a.died {
					return a.dead, nil
				}
			}
			if a.needsRest() {
				if err := a.rest(ctx); err != nil {
					return nil, err
				}
				if a.died {
					return a.dead, nil
				}
			}
			if a.carrying >= donateAfter {
				return a.donate, nil
			}
		}
	}
	return a.goHome, nil
}

// donate takes what it found to the donation room, east of Temple Square.
func (a *Adventurer) donate(ctx context.Context) (stateFn, error) {
	if a.attacked {
		return a.fightThen(a.donate), nil
	}
	if _, _, err := a.ask("recall", roomRe("Temple Square")); err != nil {
		return nil, err
	}
	if _, err := a.walk(ctx, step{"east", "Donation Room"}); err != nil {
		return nil, err
	}
	for _, kw := range a.ground.loot {
		if err := a.pause(ctx, a.cfg.Pace.Think); err != nil {
			return nil, err
		}
		if _, _, err := a.ask("drop all."+kw, dropRe); err != nil {
			return nil, err
		}
	}
	a.carrying = 0
	a.count(func(st *Stats) { st.Donations++ })
	a.log("left what it found in the donation room")
	a.say(momentDonate)
	if _, err := a.walk(ctx, step{"west", "Temple Square"}); err != nil {
		return nil, err
	}
	return a.town, nil
}

// dead: it woke in the player-death room at 1hp; recall makes sure where.
func (a *Adventurer) dead(ctx context.Context) (stateFn, error) {
	a.died, a.attacked = false, false
	a.count(func(st *Stats) { st.Deaths++ })
	a.log("died")
	return a.goHome, nil
}

// fightThen fights, then carries on to next.
func (a *Adventurer) fightThen(next stateFn) stateFn {
	return func(ctx context.Context) (stateFn, error) {
		if err := a.fight(ctx); err != nil {
			return nil, err
		}
		if a.died {
			return a.dead, nil
		}
		return next, nil
	}
}

// waitThen reads the world for d, answering tells and fighting back, then
// carries on to next.
func (a *Adventurer) waitThen(d time.Duration, next stateFn) stateFn {
	return func(ctx context.Context) (stateFn, error) {
		end := time.Now().Add(d)
		for time.Now().Before(end) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			ch, err := a.c.ReadChunk(min(time.Until(end), a.cfg.Pace.Poll))
			if err != nil {
				return nil, err
			}
			a.notice(ch)
			if a.died {
				return a.dead, nil
			}
			if a.attacked {
				return a.fightThen(next), nil
			}
			if err := a.flushTells(); err != nil {
				return nil, err
			}
		}
		return next, nil
	}
}

// ---- procedures -------------------------------------------------------------

// walk takes one step and returns the description of the room it reached.
func (a *Adventurer) walk(ctx context.Context, s step) (string, error) {
	if err := a.pause(ctx, a.cfg.Pace.Walk); err != nil {
		return "", err
	}
	text, _, err := a.ask(s.dir, roomRe(s.room))
	return text, err
}

// engage considers one kind of prey and fights it if it's a fair fight or
// easier: "a real challenge" is for a group, and a bot hasn't got one.
func (a *Adventurer) engage(ctx context.Context, p prey) error {
	if err := a.pause(ctx, a.cfg.Pace.Think); err != nil {
		return err
	}
	_, m, err := a.ask("consider "+p.keyword, considerOrGoneRe)
	if err != nil {
		return err
	}
	if m[1] == "" {
		return nil // gone: it wandered, or somebody got it
	}
	target, _ := strconv.Atoi(m[1])
	you, _ := strconv.Atoi(m[2])
	if target > you+1 {
		return nil
	}
	if !a.attacked {
		if _, _, err := a.ask("kill "+p.keyword, killRe); err != nil {
			return err
		}
	}
	return a.fight(ctx)
}

// fight lasts until Pace.Quiet goes by without a blow. It flees below
// fleeBelow -- ending up somewhere it didn't choose, which is errLost -- and
// afterwards loots the newest corpse if anything it hunts died.
func (a *Adventurer) fight(ctx context.Context) error {
	killed := 0
	last := time.Now()
	for time.Since(last) < a.cfg.Pace.Quiet {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		ch, err := a.c.ReadChunk(a.cfg.Pace.Quiet - time.Since(last))
		if err != nil {
			return err
		}
		a.notice(ch)
		if a.died {
			a.attacked = false
			return nil
		}
		if blows(ch.Text) {
			last = time.Now()
		}
		for _, name := range deaths(ch.Text) {
			if a.ground != nil && a.ground.isPrey(name) {
				killed++
			}
		}
		if strings.Contains(ch.Text, youFled) {
			a.attacked = false
			return fmt.Errorf("%w: fled", errLost)
		}
		if ch.MaxHealth > 0 && a.below(fleeBelow) {
			if err := a.c.Send("flee"); err != nil {
				return err
			}
		}
	}
	a.attacked = false
	if killed == 0 {
		return nil
	}
	a.count(func(st *Stats) { st.Kills += killed })
	a.say(momentKill)
	text, _, err := a.ask("get all from corpse", lootRe)
	if err != nil {
		return err
	}
	n := looted(text)
	a.carrying += n
	a.count(func(st *Stats) { st.Looted += n })
	return nil
}

// rest waits for regen. Regen prints nothing, so it asks for its prompt with a
// bare Enter, which repeats the newest one the server sent.
func (a *Adventurer) rest(ctx context.Context) error {
	a.log("resting at %d/%d", a.health, a.maxHealth)
	a.say(momentRest)
	for a.maxHealth > 0 && a.health*100 < restUntil*a.maxHealth {
		if err := a.pause(ctx, [2]time.Duration{a.cfg.Pace.Poll, a.cfg.Pace.Poll}); err != nil {
			return err
		}
		if _, _, err := a.ask("", anyRe); err != nil {
			return err
		}
		if a.died {
			return nil
		}
		if a.attacked {
			if err := a.fight(ctx); err != nil {
				return err
			}
			if a.died {
				return nil
			}
		}
	}
	return nil
}

// say makes a remark now and then: on one moment in talkOneIn, never twice in
// talkEvery, and only if this bot has phrases.
func (a *Adventurer) say(m moment) {
	lines := phrases[strings.ToLower(a.cfg.Name)][m]
	if len(lines) == 0 || a.rng.IntN(talkOneIn) != 0 {
		return
	}
	if !a.saidAt.IsZero() && a.now().Sub(a.saidAt) < talkEvery {
		return
	}
	a.saidAt = a.now()
	// a failure here shows up in whatever it does next
	_, _, _ = a.ask("say "+lines[a.rng.IntN(len(lines))], saidRe)
}

// ---- talking to the server ---------------------------------------------------

// ask answers any tells first, then exchanges line.
func (a *Adventurer) ask(line string, re *regexp.Regexp) (string, []string, error) {
	if err := a.flushTells(); err != nil {
		return "", nil, err
	}
	return a.exchange(line, re)
}

func (a *Adventurer) flushTells() error {
	for len(a.pendingTells) > 0 {
		who := a.pendingTells[0]
		a.pendingTells = a.pendingTells[1:]
		if _, _, err := a.exchange("tell "+who+" "+botReply, tellAnswerRe); err != nil {
			return err
		}
		a.count(func(st *Stats) { st.TellsAnswered++ })
	}
	return nil
}

// exchange sends line and reads chunks, noticing every one, until one matches
// re; it returns that chunk's text and re's submatches. Dying on the way is
// errDied; being told it's too busy fighting is errBusy; no match within
// Pace.Answer is errLost.
func (a *Adventurer) exchange(line string, re *regexp.Regexp) (string, []string, error) {
	if err := a.c.Send(line); err != nil {
		return "", nil, err
	}
	deadline := time.Now().Add(a.cfg.Pace.Answer)
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return "", nil, fmt.Errorf("%w: no answer to %q", errLost, line)
		}
		ch, err := a.c.ReadChunk(left)
		if err != nil {
			return "", nil, err
		}
		if ch.MaxHealth == 0 {
			continue // timed out; the loop's deadline decides
		}
		a.notice(ch)
		if m := re.FindStringSubmatch(ch.Text); m != nil {
			return ch.Text, m, nil
		}
		if a.died {
			return ch.Text, nil, errDied
		}
		if strings.Contains(ch.Text, busyText) {
			return ch.Text, nil, errBusy
		}
	}
}

// notice takes in a chunk: health, tells to answer, being attacked, dying.
func (a *Adventurer) notice(ch Chunk) {
	if ch.MaxHealth > 0 {
		a.health, a.maxHealth = ch.Health, ch.MaxHealth
	}
	for _, who := range tellers(ch.Text) {
		a.told(who)
	}
	if attacked(ch.Text) {
		a.attacked = true
	}
	if strings.Contains(ch.Text, youDied) {
		a.died = true
	}
}

// told queues an honest answer: once per person per tellEvery, never to itself
// or a sibling.
func (a *Adventurer) told(who string) {
	if strings.EqualFold(who, a.cfg.Name) || isSibling(who, a.cfg.Siblings) {
		return
	}
	if last, ok := a.toldAt[who]; ok && a.now().Sub(last) < tellEvery {
		return
	}
	a.toldAt[who] = a.now()
	a.pendingTells = append(a.pendingTells, who)
}

// ---- small things -------------------------------------------------------------

func (a *Adventurer) pickGround(power int) *ground {
	for i := range grounds {
		g := &grounds[i]
		if power < g.minPower || power > g.maxPower {
			continue
		}
		if until, ok := a.avoiding[g.name]; ok && a.now().Before(until) {
			continue
		}
		return g
	}
	return nil
}

func (a *Adventurer) needsRest() bool { return a.below(restBelow) }

// below: health under pct percent of max.
func (a *Adventurer) below(pct int) bool {
	return a.maxHealth > 0 && a.health*100 < pct*a.maxHealth
}

// pause waits a random while in span, or until ctx is done.
func (a *Adventurer) pause(ctx context.Context, span [2]time.Duration) error {
	d := span[0]
	if span[1] > span[0] {
		d += time.Duration(a.rng.Int64N(int64(span[1] - span[0])))
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
```

- [ ] **Step 4: Run them and see them pass, then the gate**

Run: `go test -race ./bot && make check`
Expected: PASS, green. If the fleeing test hears "flee" twice (the flee goes out once per chunk while low), that's by design; the test only reads the first.

- [ ] **Step 5: Commit**

```bash
git add bot/
git commit -m "bot: an adventurer that hunts, loots, rests and donates

A state machine over chunks: home, town, travel, hunt, donate, with
fight and rest as procedures. Considers before it kills, leaves a
ground to any player it meets, flees at a quarter health, answers a
tell once per person honestly, and talks a little.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: The adventurer against the real world

**Files:**
- Test: `bot/adventurer_game_test.go` (package `bot`)

**Interfaces:**
- Consumes: `startGame`, `createCharacter`, `loginAs` (Task 5); `NewAdventurer`, `Run`, `Stats`, `Pace`, `avoidFor` (Task 6).

- [ ] **Step 1: Write the tests**

```go
package bot

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastPace keeps the adventurer's own shape at a hundred times the speed:
// the game ticks every 10ms, a real second.
var fastPace = Pace{
	Think:  [2]time.Duration{0, 2 * time.Millisecond},
	Walk:   [2]time.Duration{0, 2 * time.Millisecond},
	Quiet:  150 * time.Millisecond,
	Poll:   20 * time.Millisecond,
	Answer: 5 * time.Second,
	Idle:   200 * time.Millisecond,
}

// runAdventurer starts one and stops it when the test ends, checking it quit
// cleanly.
func runAdventurer(t *testing.T, addr, name string) *Adventurer {
	t.Helper()
	a := NewAdventurer(AdventurerConfig{Name: name, Password: "correcthorse", Seed: 7, Pace: fastPace, Log: t.Logf})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx, addr) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Error("the adventurer didn't stop")
		}
	})
	return a
}

// The whole loop, against the real content: out to the fields, kills, loot,
// and a trip back to the donation room.
func TestAdventurer_huntsLootsAndDonates(t *testing.T) {
	addr := startGame(t, 10*time.Millisecond)
	createCharacter(t, addr, "Wren", "correcthorse")
	a := runAdventurer(t, addr, "Wren")

	require.Eventually(t, func() bool { return a.Stats().Donations >= 1 }, 90*time.Second, 50*time.Millisecond,
		"stats: %+v", a.Stats())
	st := a.Stats()
	assert.Positive(t, st.Kills)
	assert.GreaterOrEqual(t, st.Looted, donateAfter)
}

func TestAdventurer_answersATell(t *testing.T) {
	addr := startGame(t, 10*time.Millisecond)
	createCharacter(t, addr, "Wren", "correcthorse")
	createCharacter(t, addr, "Visitor", "correcthorse")
	runAdventurer(t, addr, "Wren")

	v := loginAs(t, addr, "Visitor", "correcthorse")
	require.Eventually(t, func() bool {
		if v.Send("who") != nil {
			return false
		}
		_, err := v.Expect(`(?m)^Wren `, 200*time.Millisecond)
		return err == nil
	}, 20*time.Second, 50*time.Millisecond, "Wren never came online")
	require.NoError(t, v.Send("tell Wren hello?"))
	_, err := v.Expect(`Wren tells you, "I'm a bot \(see 'help bots'\) - I can't chat, sorry!"`, 30*time.Second)
	require.NoError(t, err, v.Transcript())
}

// A player at the millpond has the Hollowfields: the bot sees them on its
// first patrol step, turns round, and waits it out in town. No pulses: at a
// hundred times speed the geese would kill the visitor in about a second, and
// they'd wake in town before the bot arrived.
func TestAdventurer_leavesAGroundToAPlayer(t *testing.T) {
	addr := startGame(t, time.Hour)
	createCharacter(t, addr, "Wren", "correcthorse")
	createCharacter(t, addr, "Visitor", "correcthorse")

	v := loginAs(t, addr, "Visitor", "correcthorse")
	require.NoError(t, recall(v))
	for _, s := range append(append([]step{}, grounds[0].route...), grounds[0].patrol[0]) {
		require.NoError(t, v.Send(s.dir))
		_, err := room(v, s.room)
		require.NoError(t, err, v.Transcript())
	}

	a := runAdventurer(t, addr, "Wren")
	require.Eventually(t, func() bool { return a.Stats().Avoided >= 1 }, 30*time.Second, 20*time.Millisecond,
		"stats: %+v", a.Stats())
	time.Sleep(500 * time.Millisecond) // a few idle rounds in town
	assert.Zero(t, a.Stats().Kills, "it never fought on a ground a player was using")
}
```

- [ ] **Step 2: Run them**

Run: `go test -race ./bot -run TestAdventurer -v 2>&1 | grep -v '^{"level"' | tail -40`
Expected: PASS. `TestAdventurer_huntsLootsAndDonates` should take seconds, not minutes. If a test fails, the `t.Logf` state lines and the Stats in the message show where the loop stopped. Fix the adventurer to match what the server really says, never the server to suit the bot.

- [ ] **Step 3: The gate, and commit**

Run: `make check`
Expected: green.

```bash
git add bot/adventurer_game_test.go
git commit -m "bot: the adventurer against the real content, at a hundred times speed

Hunts, loots and donates; answers a real player's tell; leaves the
fields to a player standing at the millpond.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: `watchmud-bots`, compose, and the docs

**Files:**
- Create: `cmd/watchmud-bots/main.go`
- Modify: `Makefile` (`build` builds it)
- Modify: `Dockerfile` (build and copy `/app/watchmud-bots`)
- Modify: `deploy/compose.yaml` (the `bots` service)
- Modify: `deploy/.env.example` (the two keys)
- Modify: `deploy/README.md` (a "## Bots" section after "## Smoke test"; the Tester flag)
- Modify: `ROADMAP.md`, `LEVELS.md`, `CLAUDE.md`

**Interfaces:**
- Consumes: `bot.NewAdventurer`, `bot.AdventurerConfig`, `bot.HumanPace`, `bot.MaxBots`, `(*Adventurer).Run`.

- [ ] **Step 1: Write `cmd/watchmud-bots/main.go`**

```go
// watchmud-bots plays the bots: one adventurer per name in WATCHMUD_BOTS, all
// sharing WATCHMUD_BOTS_PASSWORD, each logging back in after anything that
// knocks it off -- a deploy, a crash, a bad connection -- forever.
//
//	WATCHMUD_BOTS=Wren,Pim,Odo WATCHMUD_BOTS_PASSWORD=... watchmud-bots -addr watchmud:4000
//
// The characters are made by hand and flagged as bots on their records
// (deploy/README.md, "Bots"); this never creates one.
package main

import (
	"context"
	"flag"
	"hash/fnv"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/watchmud/watchmud/bot"
)

func main() {
	os.Exit(run())
}

func run() int {
	addr := flag.String("addr", "localhost:4000", "the game's telnet address")
	flag.Parse()
	log.SetFlags(log.LstdFlags)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	names := splitNames(os.Getenv("WATCHMUD_BOTS"))
	password := os.Getenv("WATCHMUD_BOTS_PASSWORD")
	if len(names) == 0 || password == "" {
		// idle rather than exit, so restart: unless-stopped doesn't spin
		log.Print("watchmud-bots: no bots configured (WATCHMUD_BOTS, WATCHMUD_BOTS_PASSWORD); idling")
		<-ctx.Done()
		return 0
	}
	if len(names) > bot.MaxBots {
		log.Printf("watchmud-bots: %d bots, but the server allows %d connections from one address", len(names), bot.MaxBots)
		return 2
	}

	var wg sync.WaitGroup
	for i, name := range names {
		wg.Go(func() {
			siblings := without(names, name)
			// staggered, so a deploy doesn't bring them all in at once
			if !sleep(ctx, time.Duration(i)*45*time.Second) {
				return
			}
			superviseForever(ctx, *addr, bot.AdventurerConfig{
				Name:     name,
				Password: password,
				Siblings: siblings,
				Seed:     seedFor(name),
				Pace:     bot.HumanPace,
				Log:      log.Printf,
			})
		})
	}
	wg.Wait()
	log.Print("watchmud-bots: stopped")
	return 0
}

// superviseForever runs one adventurer, and after anything that stops it, waits
// 30 seconds to 2 minutes and runs it again.
func superviseForever(ctx context.Context, addr string, cfg bot.AdventurerConfig) {
	for {
		err := bot.NewAdventurer(cfg).Run(ctx, addr)
		if ctx.Err() != nil {
			return
		}
		wait := 30*time.Second + time.Duration(rand.Int64N(int64(90*time.Second)))
		log.Printf("%s: %v; back in %s", cfg.Name, err, wait.Round(time.Second))
		if !sleep(ctx, wait) {
			return
		}
	}
}

func splitNames(s string) []string {
	var out []string
	for _, n := range strings.Split(s, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func without(names []string, name string) []string {
	var out []string
	for _, n := range names {
		if n != name {
			out = append(out, n)
		}
	}
	return out
}

// seedFor gives each bot its own dice, the same ones every run.
func seedFor(name string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return h.Sum64()
}

// sleep waits d, or returns false if ctx ends first.
func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
```

Note: a new run of `superviseForever` gets a fresh `Adventurer`, which forgets avoided grounds and tell history. That's fine: a reconnect is rare, and forgetting only makes it a little chattier once.

- [ ] **Step 2: Build it, and wire it into the Makefile and the image.** `Makefile` `build:` gains a third line, `$(GO) build -o $(BIN_DIR)/watchmud-bots ./cmd/watchmud-bots`, and its `##` comment becomes `## build: compile bin/watchmud, bin/watchmud-bot (the smoke test) and bin/watchmud-bots`.

`Dockerfile`, after the watchmud-bot build line:

```dockerfile
# the inhabitants: the bots service in deploy/compose.yaml runs this
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/watchmud-bots ./cmd/watchmud-bots
```

and after `COPY --from=build /out/watchmud-bot /app/watchmud-bot`:

```dockerfile
COPY --from=build /out/watchmud-bots /app/watchmud-bots
```

Run: `make build && bin/watchmud-bots & sleep 1; kill %1`
Expected: `watchmud-bots: no bots configured (WATCHMUD_BOTS, WATCHMUD_BOTS_PASSWORD); idling`, then `watchmud-bots: stopped` on the kill.

- [ ] **Step 3: The compose service.** In `deploy/compose.yaml`, after the `watchmud` service and before `mongo`:

```yaml
  # The bots: always-on characters a program plays, labelled [bot] in who.
  # Each is made by hand and flagged on its record first -- README, "Bots".
  # With WATCHMUD_BOTS empty this idles.
  bots:
    image: ghcr.io/watchmud/watchmud:${WATCHMUD_VERSION:?run deploy/deploy.sh <version> -- see deploy/README.md}
    restart: unless-stopped
    depends_on:
      - watchmud
    entrypoint: ["/app/watchmud-bots", "-addr", "watchmud:4000"]
    environment:
      WATCHMUD_BOTS: ${WATCHMUD_BOTS:-}
      WATCHMUD_BOTS_PASSWORD: ${WATCHMUD_BOTS_PASSWORD:-}
    logging:
      driver: json-file
      options:
        max-size: 20m
        max-file: "5"
```

In `deploy/.env.example`, after `WATCHMUD_PORT=4000`:

```sh

# The bots (README, "Bots"): comma-separated names of characters made by hand
# and flagged as bots, all with this one password. Empty: no bots.
WATCHMUD_BOTS=
WATCHMUD_BOTS_PASSWORD=
# The deploy smoke test's character (README, "Smoke test").
WATCHMUD_BOT_PASSWORD=
```

Run: `docker compose --env-file deploy/.env.example -f deploy/compose.yaml config --quiet 2>&1 | head -3; WATCHMUD_VERSION=x WATCHMUD_DB_PASSWORD=x docker compose -f deploy/compose.yaml config --quiet && echo compose-ok`
Expected: `compose-ok`.

- [ ] **Step 4: Try the whole service locally** against the image, like the smoke test's deploy check: `make docker-build`, then in the scratchpad a compose project with `watchmud` (memstore config, no TLS) and a `bots` service from `watchmud:dev` with the bots entrypoint. Create `Wren` and `Pim` over telnet at `localhost:4999`, set `WATCHMUD_BOTS=Wren,Pim` and the password, `up -d bots`, and watch `docker compose logs -f bots` for 3 minutes.
Expected: `Wren: logged in`, `Wren: off to the Hollowfields at power 1`, `Wren: hunting the Hollowfields`, then fights and at least one `resting` or `left what it found`; Pim's lines start about 45s later. Then `docker compose restart watchmud`. Expected: each bot logs an error and `back in ...`, then `logged in` again. Tear the project down.

- [ ] **Step 5: Docs.** `deploy/README.md`, after the "## Smoke test" section:

```markdown
## Bots

The `bots` service plays a few characters around the clock: they walk out to the
Hollowfields, fight what's a fair fight, loot, rest, and leave what they find in the
donation room. They're labelled `[bot]` in `who`, `help bots` explains them, and a
tell to one gets an automatic honest answer. They leave a hunting ground to any player
they meet there.

Each bot is a character made once by hand -- the bots never create one:

1. `telnet watchmud.com 4000`, create it (`Wren`, say), `quit`. Same password for all.
2. Flag it, while it's logged out, so `who` says what it is:

   ```sh
   docker compose -f deploy/compose.yaml exec mongo mongosh -u root -p \
     --authenticationDatabase admin watchmud \
     --eval 'db.players.updateOne({name: "Wren"}, {$set: {bot: true}})'
   ```

   Do the same for the smoke test's character (Tester): it's a bot too.
3. In `deploy/.env`: `WATCHMUD_BOTS=Wren,Pim,Odo` and `WATCHMUD_BOTS_PASSWORD=...`,
   then `docker compose -f deploy/compose.yaml up -d bots`.

At most 5: they all connect from the one container, and the game allows 5
connections per address. `docker compose -f deploy/compose.yaml logs -f bots` shows
what they're up to, a line per change of plan. Every deploy restarts them onto the
new version; they log back in within a couple of minutes.
```

`LEVELS.md`, in the table that has the Regen rows, add after them:

```markdown
| Dropped decay | 30m (`rules.DroppedDecay`) | how long a donation waits for a newbie |
```

`ROADMAP.md`, after the smoke-test bullet under "Then: content and bots":

```markdown
- ~~**Inhabitants.**~~ Done 2026-09-30: `bot.Adventurer` and `cmd/watchmud-bots`, the
  `bots` compose service -- always-on, labelled `[bot]` in `who` from a flag on the
  record, hunting the Hollowfields, donating what they find, and honest when told
  to. Dropped items decay after 30 minutes, so the donation room turns over.
  Future: a **Wanderer** (roams, fights only when attacked, never loots), a
  **Socialite** (greets new characters in town, answers newbie questions), bots that
  **explore and map** the world instead of following hand-written hunting grounds
  (needs room names to stay unique -- true today, not enforced), and bots that **wear
  the upgrades they find** and grow into the Barrow.
```

`CLAUDE.md`:
- In the Commands block, change the `make build` comment to `# -> bin/watchmud, bin/watchmud-bot, bin/watchmud-bots`.
- In "### Loot and corpses", after the sentence ending "`World.DecayCorpses` runs on the mobile pulse and removes them, contents and all, after `rules.CorpseDecay`. The zero `DecaysAt` means never.", change `DecayCorpses` to `DecayFloors` and add: "Anything a character drops gets a `DecaysAt` too (`rules.DroppedDecay`, 30 minutes), and `get` clears it -- so the donation room the bots fill turns over. Zone resets and wizard `load`s never go through `drop`."
- In Conventions, the bullet on wizard commands gets a sentence at the end: "`Bot` on the record is the same kind of hand-set flag (`make bot NAME=...`); it only labels the character in `who`, and nothing may branch on it."
- Under `### bot/`, add a paragraph:

```markdown
`bot.Adventurer` is the inhabitant: a state machine (goHome, town, travel, hunt,
donate; fight and rest are procedures) driven by `Client.ReadChunk`, one prompt's
worth of output at a time. Its manners are constants, not config: it never fights
in a room with a real player (siblings are known by name) and leaves that ground for
10 minutes; `consider` gates every kill; it answers each person's tell once per 10
minutes with the same honest line. Hunting grounds (`bot/grounds.go`) are hand-written
tables walked against the real content by `TestGrounds_walk`; never add the
hedge-witch as prey. `cmd/watchmud-bots` runs up to 5 (one address, the server's
cap), reconnecting forever.
```

- [ ] **Step 6: The gate, and commit**

Run: `make check && make docker-build >/dev/null && docker run --rm --entrypoint /app/watchmud-bots watchmud:dev & sleep 2; docker ps --filter ancestor=watchmud:dev -q | xargs -r docker stop`
Expected: `make check` green; the container logs `no bots configured ...; idling`.

```bash
git add cmd/watchmud-bots Makefile Dockerfile deploy/ ROADMAP.md LEVELS.md CLAUDE.md
git commit -m "watchmud-bots: the inhabitants as a compose service

One adventurer per name in WATCHMUD_BOTS, reconnecting forever, idle
with none configured. README: making each bot by hand, flagging it
(Tester too), and the .env keys. ROADMAP: inhabitants done, with the
Wanderer, the Socialite, mapping and gear as future work.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
