# Bots that buy their kit -- Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **This one is hand-implemented, with a split:** Claude writes the tests and the docs.
> The user writes the Go production code in `bot/`, and Claude walks through each change
> first -- what it does, why it's shaped that way, what it touches -- then reviews it
> before the commit. The Go below is the reference the walkthrough explains, not
> something to paste.

**Goal:** A hunter bot spends its coins on the General Store's power-4 kit, a piece at a
time, wears it, keeps it repaired at the smithy, and donates the starter pieces it took
off.

**Architecture:** `bot/outfit.go` holds the kit table and the readers for the text a
player sees (equipment rows, the purse, the store's two answers). `town` reads the
equipment and the purse and, when there's a repair to make or a piece it can afford,
goes on an `outfit` visit -- smithy, store, back to Market Square -- before the rest of
what `town` did, now `setOut`, picks a ground. Prices are learned from the store's
answers and kept in memory. The donation trip also drops the starter pieces by alias.

**Tech Stack:** Go, testify, the bot's telnet client, the real content in-process.

**Spec:** `docs/superpowers/specs/2026-10-15-bots-buy-their-kit-design.md`

## Global Constraints

- `make check` green before every commit; one commit per task, on master.
- The bot imports nothing of the server; it reads rendered text. Every pattern is
  anchored to the start of a line (`(?m)^`). Fix a bot pattern, never the renderer.
- Kit, in order: arming sword (wielded, replaces "training"), quilted jerkin (body,
  replaces "wool"), studded cap (head), studded boots (feet), riding gloves (hands),
  oilskin cloak (about body, replaces "travelers").
- `buy` and `wear` use the piece's full name. A replaced piece is named by its alias,
  never "cloak".
- Repair mark: `broken`, or `n/m` with `3n <= m`.
- No prices or durabilities written into the bot.
- Only hunters outfit. Grounds, `consider`'s rule, `keepOut` unchanged.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **A repair it can't afford, every visit.** A bot with gear at the mark and too few
   coins must not walk to the smithy forever doing nothing: it goes only with coins in
   its purse, and one visit per town visit at most (`outfit` goes on to `setOut`, never
   back to `town`). Pinned in Task 3 (`TestWantsRepair`).
2. **A slot held by something that isn't the starter piece.** If anything but the piece
   `replaces` names is in the slot, the bot mustn't buy a piece it can't wear -- and buy
   it again every visit. `nextPiece` skips such a slot. Pinned in Task 1
   (`TestNextPiece_skipsASlotItCantClear`).
3. **Learning a price it can't afford.** The first visit to a piece of unknown price
   tries to buy it; refused, the price is kept and the bot doesn't go back until the
   purse covers it. Pinned in Task 2 (`TestShop_buysWearsAndLearns`) and Task 3
   (`TestAffordable`).
4. **Donating a starter piece still worn.** `drop all.training` with the dagger on
   answers "You'll have to remove it first." -- an answer, not a lost bot. Pinned in
   Task 4.
5. **The town state no longer starting with `equipment` alone.** Pipe tests that drive
   `town` see `inventory` too; Task 3 runs the whole `bot` package, not just the new
   tests.

---

## File map

- Create `bot/outfit.go` -- the kit, the readers, `nextPiece`, `needsRepair`, the
  `outfit` state and `shop`.
- Create `bot/outfit_test.go` -- readers and decisions (literal text), `shop` on a pipe.
- Create `bot/outfit_game_test.go` -- the real content: the kit against the store and
  the starting gear, the real text, the route, a hunter buying, a hunter repairing.
- Modify `bot/adventurer.go` -- `town` split into `town` + `setOut`; `prices`; `Stats`
  gains `Bought`, `Repairs`, `Power`; `donate` drops spares; `dropRe` takes "remove it
  first".
- Modify `bot/game_test.go` -- `startGameStore` hands back the memstore; `setRecord`.
- Docs: CLAUDE.md (bot/ section), `telnet/help.go` (`help bots`), docs/working.md
  ("On master"), ROADMAP.md (bots line).

---

### Task 1: The kit and what the bot reads

**Files:**
- Create: `bot/outfit.go`
- Create: `bot/outfit_test.go`
- Create: `bot/outfit_game_test.go`
- Modify: `bot/game_test.go`

**Interfaces:**
- Produces:
  - `type piece struct{ name, slot, replaces string }`, `var kit []piece`
  - `type worn struct { slot, item string; power, durability, maxDurability int; broken bool }`
  - `func readEquipment(text string) []worn`
  - `func readPurse(text string) int`
  - `func (w worn) needsRepair() bool`
  - `func nextPiece(rows []worn) *piece`
  - `func slotHolds(rows []worn, slot string) (worn, bool)`
  - `func spares() []string` -- every non-empty `replaces`, in kit order
  - regexps `inventoryRe`, `buyRe` (m[1] bought price, m[2] refused price),
    `repairRe`, `removedRe`, `woreRe`
  - test helpers: `startGameStore(t, tick) (string, *memstore.Store)`,
    `setRecord(t, store, name, func(*player.Record))`

- [ ] **Step 1: Write the failing tests (Claude)**

`bot/outfit_test.go`:

```go
package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// What the equipment list looks like with color off: a padded table under its
// heading, a slot of one or two words, condition last or not at all.
const starterEquipment = "Equipment (power 1)\n" +
	"  wielded     a training dagger  power 1  25/25\n" +
	"  body        a coarse wool tunic  power 1  20/20\n" +
	"  about body  a travelers cloak  power 1  6/20\n" +
	"  neck        a temple token     power 1\n"

func TestReadEquipment(t *testing.T) {
	rows := readEquipment(starterEquipment)
	require.Len(t, rows, 4)
	assert.Equal(t, worn{slot: "wielded", item: "a training dagger", power: 1, durability: 25, maxDurability: 25}, rows[0])
	assert.Equal(t, "about body", rows[2].slot, "a slot of two words")
	assert.Equal(t, worn{slot: "neck", item: "a temple token", power: 1}, rows[3], "no condition: never wears out")

	broken := readEquipment("Equipment (power 0)\n  wielded  an arming sword  power 4  broken\n")
	require.Len(t, broken, 1)
	assert.True(t, broken[0].broken)

	assert.Empty(t, readEquipment("Nothing equipped.\n"))
}

func TestNeedsRepair_atTheYellowMark(t *testing.T) {
	assert.False(t, worn{durability: 7, maxDurability: 20}.needsRepair(), "more than a third left")
	assert.True(t, worn{durability: 6, maxDurability: 20}.needsRepair(), "a third or less")
	assert.True(t, worn{broken: true, maxDurability: 20}.needsRepair())
	assert.False(t, worn{}.needsRepair(), "never wears out")
}

func TestReadPurse(t *testing.T) {
	assert.Equal(t, 37, readPurse("Inventory\n  a waterskin  power 1\nYou have 37 coins.\n"))
	assert.Equal(t, 1, readPurse("Inventory\n  a waterskin  power 1\nYou have 1 coin.\n"))
	assert.Zero(t, readPurse("Inventory\n  a waterskin  power 1\n"), "an empty purse says nothing")
	assert.Zero(t, readPurse(`Bob says, "You have 900 coins."`), "only its own purse line")
}

func TestNextPiece(t *testing.T) {
	rows := readEquipment(starterEquipment)
	require.NotNil(t, nextPiece(rows))
	assert.Equal(t, "arming sword", nextPiece(rows).name, "the starter dagger is the starter: replace it")

	rows[0] = worn{slot: "wielded", item: "an arming sword", power: 4}
	assert.Equal(t, "quilted jerkin", nextPiece(rows).name)

	full := []worn{}
	for _, p := range kit {
		full = append(full, worn{slot: p.slot, item: "a " + p.name, power: 4})
	}
	assert.Nil(t, nextPiece(full), "the whole kit on")
}

// A slot with something in it that the kit doesn't replace -- not its starter
// piece -- can't be cleared by the alias it knows, so buying for it would buy a
// piece it can't wear, again every visit.
func TestNextPiece_skipsASlotItCantClear(t *testing.T) {
	rows := []worn{
		{slot: "wielded", item: "an arming sword", power: 4},
		{slot: "body", item: "a quilted jerkin", power: 4},
		{slot: "head", item: "a bandit hood", power: 4},
	}
	assert.Equal(t, "studded boots", nextPiece(rows).name)
}

func TestReplies(t *testing.T) {
	m := buyRe.FindStringSubmatch("You buy an arming sword for 80 coins.\n")
	require.NotNil(t, m)
	assert.Equal(t, []string{"80", ""}, m[1:])
	m = buyRe.FindStringSubmatch("A quilted jerkin would cost 80 coins, and you have 12 coins.\n")
	require.NotNil(t, m)
	assert.Equal(t, []string{"", "80"}, m[1:])
	assert.Nil(t, buyRe.FindStringSubmatch(`Bob says, "You buy an arming sword for 1 coin."`))

	for _, said := range []string{
		"The smith works on a training dagger until it's as good as new, for 8 coins.\n",
		"A training dagger would cost 8 coins, and you have 0 coins.\n",
		"That doesn't need repairing.\n",
	} {
		assert.True(t, repairRe.MatchString(said), said)
	}
	assert.Equal(t, []string{"training", "wool", "travelers"}, spares())
}
```

`bot/game_test.go` -- `startGame` becomes a wrapper, and a record can be edited while
the character is logged out (the logout saves before the goodbye, so after
`createCharacter` returns the record is the one to edit):

```go
func startGame(t *testing.T, tick time.Duration) string {
	t.Helper()
	addr, _ := startGameStore(t, tick)
	return addr
}

// startGameStore is startGame, handing back the store too, so a test can give
// a character what it needs while they're logged out.
func startGameStore(t *testing.T, tick time.Duration) (string, *memstore.Store) {
	// ... the old body of startGame, returning ln.Addr().String(), store
}

// setRecord changes a logged-out character's record.
func setRecord(t *testing.T, store *memstore.Store, name string, change func(*player.Record)) {
	t.Helper()
	rec, ok, err := store.Load(name)
	require.NoError(t, err)
	require.True(t, ok, "no record for %s", name)
	change(rec)
	require.NoError(t, store.Save(rec))
}
```

`bot/outfit_game_test.go`:

```go
package bot

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/player"
)

// The kit against the real content: every piece something the General Store
// sells under that name, and every alias it removes by on a piece of the
// starting kit, in its short description (so the equipment list shows which
// is on) and on no piece of the kit (so removing it never takes off a
// bought one).
func TestKit_theStoreSellsIt(t *testing.T) {
	content, err := loader.LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	shop := content.Zones["wrathrock"].Shops["general_store"]
	require.NotNil(t, shop)
	for _, p := range kit {
		found := false
		for _, s := range shop.Stock {
			if s.Object.Name == p.name {
				found = true
			}
		}
		assert.True(t, found, "the store doesn't sell %q", p.name)
	}
	for _, p := range kit {
		if p.replaces == "" {
			continue
		}
		found := false
		for _, g := range content.Catalog.StartingGear {
			d := content.Zones[g.ZoneId].ObjectDefinitions[g.DefinitionId]
			if d.HasAlias(p.replaces) && strings.Contains(d.ShortDescription, p.replaces) {
				found = true
			}
		}
		assert.True(t, found, "%q names nothing in the starting kit", p.replaces)
		for _, q := range kit {
			for _, s := range shop.Stock {
				if s.Object.Name == q.name {
					assert.False(t, s.Object.HasAlias(p.replaces), "%q also takes off %s", p.replaces, q.name)
				}
			}
		}
	}
}

// The text the readers read, as the real server writes it.
func TestOutfit_readsTheRealText(t *testing.T) {
	addr, store := startGameStore(t, time.Hour)
	createCharacter(t, addr, "Quillon", "correcthorse")
	setRecord(t, store, "Quillon", func(r *player.Record) { r.Coins = 100 })
	c := loginAs(t, addr, "Quillon", "correcthorse")
	require.NoError(t, recall(c))

	require.NoError(t, c.Send("equipment"))
	m, err := c.Expect(`(?s)(Equipment \(power \d+\)\n.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	rows := readEquipment(m[1])
	require.Len(t, rows, 4, m[1])
	assert.Equal(t, "arming sword", nextPiece(rows).name)

	require.NoError(t, c.Send("inventory"))
	m, err = c.Expect(`(?s)((?:Inventory|You aren't carrying anything\.)\n.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 100, readPurse(m[1]))

	for _, s := range []step{waitIn, {"west", "Smithy"}} {
		require.NoError(t, c.Send(s.dir))
		_, err := room(c, s.room)
		require.NoError(t, err)
	}
	require.NoError(t, c.Send("repair all"))
	m, err = c.Expect(`(?s)(.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	assert.True(t, repairRe.MatchString(m[1]), m[1])

	for _, s := range []step{{"east", "Market Square"}, {"east", "General Store"}} {
		require.NoError(t, c.Send(s.dir))
		_, err := room(c, s.room)
		require.NoError(t, err)
	}
	require.NoError(t, c.Send("buy arming sword"))
	m, err = c.Expect(`(?s)(.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	bought := buyRe.FindStringSubmatch(m[1])
	require.NotNil(t, bought, m[1])
	assert.NotEmpty(t, bought[1])
	require.NoError(t, c.Send("buy quilted jerkin"))
	m, err = c.Expect(`(?s)(.*?)`+prompt, 5*time.Second)
	require.NoError(t, err)
	refused := buyRe.FindStringSubmatch(m[1])
	require.NotNil(t, refused, m[1])
	assert.NotEmpty(t, refused[2], "100 coins buys one piece, not two")

	require.NoError(t, c.Send("remove training"))
	_, err = c.Expect(removedRe.String(), 5*time.Second)
	require.NoError(t, err)
	require.NoError(t, c.Send("wear arming sword"))
	_, err = c.Expect(woreRe.String(), 5*time.Second)
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./bot -run 'TestReadEquipment|TestNeedsRepair|TestReadPurse|TestNextPiece|TestReplies|TestKit_|TestOutfit_readsTheRealText'`
Expected: build failure -- `readEquipment`, `kit`, `buyRe` ... undefined.

- [ ] **Step 3: Walk through `bot/outfit.go`, then the user writes it**

The reference:

```go
package bot

import (
	"regexp"
	"strconv"
	"strings"
)

// piece is one thing a hunter buys at the General Store: its full name, which
// buy and wear take ("cap" alone would buy the leather cap, earlier in the
// stock); the slot the equipment list shows it in; and an alias of the
// starter piece it replaces, a word of that piece's short description, or ""
// for a slot the starting kit leaves empty. TestKit_theStoreSellsIt holds it
// to the real content.
type piece struct{ name, slot, replaces string }

// kit is what a hunter buys, in order: power 2 after the jerkin, 3 after the
// gloves, and no further -- the temple token is power 1 and stays on.
var kit = []piece{
	{"arming sword", "wielded", "training"},
	{"quilted jerkin", "body", "wool"},
	{"studded cap", "head", ""},
	{"studded boots", "feet", ""},
	{"riding gloves", "hands", ""},
	{"oilskin cloak", "about body", "travelers"},
}

// worn is one row of the equipment list. Durability and maxDurability are both
// zero for gear that never wears out.
type worn struct {
	slot, item                string
	power                     int
	durability, maxDurability int
	broken                    bool
}

var (
	// a row: two spaces in, columns two or more spaces apart, the condition
	// last or not there at all
	equipmentRowRe = regexp.MustCompile(`(?m)^  (\S+(?: \S+)?)  +(.+?)  +power (\d+)(?:  +(?:(broken)|(\d+)/(\d+)))?$`)
	purseRe        = regexp.MustCompile(`(?m)^You have (\d+) coins?\.$`)
	inventoryRe    = regexp.MustCompile(`(?m)^(?:Inventory|You aren't carrying anything\.)$`)
	// what the store says to a buy: bought, at a price, or what it would cost
	buyRe     = regexp.MustCompile(`(?m)^(?:You buy .+ for (\d+) coins?\.|[A-Z].* would cost (\d+) coins?, and you have \d+ coins?\.)$`)
	repairRe  = regexp.MustCompile(`(?m)^(?:The smith works on |[A-Z].* would cost \d+ coins?, and you have |That doesn't need repairing\.$)`)
	removedRe = regexp.MustCompile(`(?m)^You stop using .+\.$`)
	woreRe    = regexp.MustCompile(`(?m)^You wear .+\.$`)
)

func readEquipment(text string) []worn {
	var rows []worn
	for _, m := range equipmentRowRe.FindAllStringSubmatch(text, -1) {
		w := worn{slot: m[1], item: m[2], broken: m[4] != ""}
		w.power, _ = strconv.Atoi(m[3])
		w.durability, _ = strconv.Atoi(m[5])
		w.maxDurability, _ = strconv.Atoi(m[6])
		rows = append(rows, w)
	}
	return rows
}

// readPurse is the coins the inventory says it has; no line is none.
func readPurse(text string) int {
	m := purseRe.FindStringSubmatch(text)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// needsRepair: broken, or down to a third -- the mark that turns yellow for a
// player.
func (w worn) needsRepair() bool {
	return w.broken || (w.maxDurability > 0 && 3*w.durability <= w.maxDurability)
}

func slotHolds(rows []worn, slot string) (worn, bool) {
	for _, r := range rows {
		if r.slot == slot {
			return r, true
		}
	}
	return worn{}, false
}

// nextPiece is the first piece of the kit not worn whose slot is free or holds
// the starter piece it replaces; nil when there's none.
func nextPiece(rows []worn) *piece {
	for i := range kit {
		p := &kit[i]
		r, taken := slotHolds(rows, p.slot)
		switch {
		case !taken:
			return p
		case strings.HasSuffix(r.item, " "+p.name):
			continue // on already
		case p.replaces != "" && strings.Contains(r.item, p.replaces):
			return p
		}
		// something else is there, and nothing it knows takes it off
	}
	return nil
}

// spares is what a hunter has taken off and gives away: the starter pieces,
// by alias.
func spares() []string {
	var out []string
	for _, p := range kit {
		if p.replaces != "" {
			out = append(out, p.replaces)
		}
	}
	return out
}
```

Walkthrough points: why full names (`Matches` is name-or-alias exact); why the
two-spaces rule finds columns (`telnet/table.go` puts at least two spaces between
cells, and an item's name has single ones); why the purse is "none" without a line
(`renderPurse` says nothing for zero); why `[A-Z]` on the would-cost lines (the item is
capitalized, and a player's line starts with their name -- "Bob says" -- which can't
be followed by "would cost").

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./bot -run 'TestReadEquipment|TestNeedsRepair|TestReadPurse|TestNextPiece|TestReplies|TestKit_|TestOutfit_readsTheRealText'`
Expected: PASS. If `TestOutfit_readsTheRealText` fails on the rows, its message is the
real equipment text: fix `equipmentRowRe`, not the renderer.

- [ ] **Step 5: Review, `make check`, commit**

```bash
git add bot/outfit.go bot/outfit_test.go bot/outfit_game_test.go bot/game_test.go
git commit -m "Bots: the kit they'll buy, and reading equipment, purse and the store

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: `shop` buys, wears and learns prices

**Files:**
- Modify: `bot/outfit.go`, `bot/adventurer.go` (fields, `Stats`)
- Test: `bot/outfit_test.go`

**Interfaces:**
- Consumes: `kit`, `nextPiece`, `slotHolds`, `buyRe`, `removedRe`, `woreRe` (Task 1)
- Produces:
  - `Adventurer.prices map[string]int` -- piece name to coins, made in `NewAdventurer`
  - `Stats.Bought`, `Stats.Repairs`, `Stats.Power int`
  - `func (a *Adventurer) affordable(rows []worn, coins int) *piece`
  - `func (a *Adventurer) shop(ctx context.Context, rows []worn, coins int) error`

- [ ] **Step 1: Write the failing tests (Claude)**

Add to `bot/outfit_test.go` (imports `context`):

```go
func TestAffordable(t *testing.T) {
	a, _ := adventurer(t)
	rows := readEquipment(starterEquipment)
	assert.Equal(t, "arming sword", a.affordable(rows, 0).name, "a price it doesn't know is worth asking")
	a.prices["arming sword"] = 80
	assert.Nil(t, a.affordable(rows, 79), "known, and short")
	assert.NotNil(t, a.affordable(rows, 80))
}

// Two pieces in a row while the purse lasts: buy, take off what's in the
// slot, put on what it bought; then a price it learns by being refused.
func TestShop_buysWearsAndLearns(t *testing.T) {
	a, g := adventurer(t)
	a.prices["quilted jerkin"] = 80
	rows := readEquipment(starterEquipment)
	done := make(chan error, 1)
	go func() { done <- a.shop(context.Background(), rows, 170) }()

	assert.Equal(t, "buy arming sword", g.heard())
	g.say("You buy an arming sword for 80 coins.\r\n<100/100hp> ")
	assert.Equal(t, "remove training", g.heard())
	g.say("You stop using a training dagger.\r\n<100/100hp> ")
	assert.Equal(t, "wear arming sword", g.heard())
	g.say("You wear an arming sword.\r\n<100/100hp> ")

	assert.Equal(t, "buy quilted jerkin", g.heard(), "90 left covers the jerkin's 80")
	g.say("You buy a quilted jerkin for 80 coins.\r\n<100/100hp> ")
	assert.Equal(t, "remove wool", g.heard())
	g.say("You stop using a coarse wool tunic.\r\n<100/100hp> ")
	assert.Equal(t, "wear quilted jerkin", g.heard())
	g.say("You wear a quilted jerkin.\r\n<100/100hp> ")

	assert.Equal(t, "buy studded cap", g.heard(), "10 left, and the cap's price unknown: ask")
	g.say("A studded cap would cost 80 coins, and you have 10 coins.\r\n<100/100hp> ")
	require.NoError(t, <-done)

	assert.Equal(t, 80, a.prices["studded cap"], "learned by being refused")
	assert.Equal(t, 2, a.Stats().Bought)
}

// The cap's slot is empty: nothing to take off first.
func TestShop_aFreeSlot(t *testing.T) {
	a, g := adventurer(t)
	rows := []worn{
		{slot: "wielded", item: "an arming sword", power: 4},
		{slot: "body", item: "a quilted jerkin", power: 4},
	}
	a.prices["studded boots"] = 80
	done := make(chan error, 1)
	go func() { done <- a.shop(context.Background(), rows, 80) }()

	assert.Equal(t, "buy studded cap", g.heard())
	g.say("You buy a studded cap for 80 coins.\r\n<100/100hp> ")
	assert.Equal(t, "wear studded cap", g.heard())
	g.say("You wear a studded cap.\r\n<100/100hp> ")
	require.NoError(t, <-done, "nothing left for the boots' known 80")
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./bot -run 'TestAffordable|TestShop_'`
Expected: build failure -- `a.prices`, `a.affordable`, `a.shop` undefined.

- [ ] **Step 3: Walk through, then the user writes it**

`bot/adventurer.go`: a field `prices map[string]int // what each piece of the kit costs, as the store said` beside `avoiding`, made in `NewAdventurer`; `Stats` gains

```go
	// Bought is pieces of kit bought, Repairs visits to the smith; Power is
	// the power the equipment list last said.
	Bought, Repairs, Power int
```

`bot/outfit.go`:

```go
// affordable is the next piece of the kit if the purse covers what it's known
// to cost -- or its price isn't known yet, and asking is how to learn it.
func (a *Adventurer) affordable(rows []worn, coins int) *piece {
	p := nextPiece(rows)
	if p == nil {
		return nil
	}
	if price, known := a.prices[p.name]; known && coins < price {
		return nil
	}
	return p
}

// shop is at the General Store: it buys down the kit while the purse lasts,
// takes off the starter piece in the way, and wears what it bought. A refusal
// ends it, with the price learned.
func (a *Adventurer) shop(ctx context.Context, rows []worn, coins int) error {
	for {
		p := a.affordable(rows, coins)
		if p == nil {
			return nil
		}
		if err := a.pause(ctx, a.cfg.Pace.Think); err != nil {
			return err
		}
		_, m, err := a.ask("buy "+p.name, buyRe)
		if err != nil {
			return err
		}
		if m[1] == "" {
			a.prices[p.name], _ = strconv.Atoi(m[2])
			a.log("%s costs %d; saving up", p.name, a.prices[p.name])
			return nil
		}
		price, _ := strconv.Atoi(m[1])
		a.prices[p.name] = price
		coins -= price
		if _, taken := slotHolds(rows, p.slot); taken {
			if _, _, err := a.ask("remove "+p.replaces, removedRe); err != nil {
				return err
			}
		}
		if _, _, err := a.ask("wear "+p.name, woreRe); err != nil {
			return err
		}
		// on now: what was in its slot is off, so the loop moves down the kit
		rows = append(slices.DeleteFunc(slices.Clone(rows), func(r worn) bool { return r.slot == p.slot }),
			worn{slot: p.slot, item: "a " + p.name, power: 4})
		a.count(func(st *Stats) { st.Bought++ })
		a.log("bought and wore %s for %d coins", p.name, price)
	}
}
```

Walkthrough points: `rows` is updated in place of re-reading `equipment` after every
piece -- the starter's row goes, the bought one comes -- so `nextPiece` moves down the
kit (`slices` joins the imports); `remove` only when the slot is taken, which
`nextPiece` has already said means the starter piece; and an unexpected answer is left
to `ask`'s `errLost`, like everywhere else.

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./bot -run 'TestAffordable|TestShop_'`
Expected: PASS.

- [ ] **Step 5: Review, `make check`, commit**

```bash
git add bot/outfit.go bot/outfit_test.go bot/adventurer.go
git commit -m "Bots: shop buys down the kit, wears it, and learns what it costs

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: The outfit visit from town

**Files:**
- Modify: `bot/adventurer.go` (`town`, new `setOut`)
- Modify: `bot/outfit.go` (`wantsRepair`, `outfit`)
- Test: `bot/outfit_test.go`, `bot/outfit_game_test.go`

**Interfaces:**
- Consumes: `readEquipment`, `readPurse`, `inventoryRe`, `repairRe` (Task 1);
  `affordable`, `shop`, `prices`, `Stats` (Task 2)
- Produces:
  - `func (a *Adventurer) setOut(ctx context.Context, power int) (stateFn, error)`
  - `func wantsRepair(rows []worn, coins int) bool`
  - `func (a *Adventurer) outfit(rows []worn, coins int) stateFn`

- [ ] **Step 1: Write the failing tests (Claude)**

`bot/outfit_test.go`:

```go
// Only with coins to pay: a bot with worn gear and an empty purse would walk
// to the smith every visit to be told what it can't afford.
func TestWantsRepair(t *testing.T) {
	rows := readEquipment(starterEquipment) // the cloak is at 6/20
	assert.True(t, wantsRepair(rows, 10))
	assert.False(t, wantsRepair(rows, 0))
	assert.False(t, wantsRepair(readEquipment(starterEquipment)[:2], 10), "nothing at the mark")
}
```

`bot/outfit_game_test.go`:

```go
// A hunter with 200 coins buys the sword and the jerkin -- 80 each -- on its
// way out, is refused the cap, and is power 2. And then it goes hunting.
func TestAdventurer_buysItsKit(t *testing.T) {
	addr, store := startGameStore(t, 10*time.Millisecond)
	createCharacter(t, addr, "Wren", "correcthorse")
	setRecord(t, store, "Wren", func(r *player.Record) { r.Coins = 200 })
	a := runAdventurer(t, addr, "Wren")

	ok := assert.Eventually(t, func() bool { return a.Stats().Kills >= 1 }, 60*time.Second, 50*time.Millisecond)
	st := a.Stats()
	require.True(t, ok, "stats: %+v", st)
	assert.Equal(t, 2, st.Bought)
	assert.Equal(t, 2, st.Power, "sword 4, jerkin 4, cloak 1, token 1")
}

// A dagger worn to a fifth goes to the smith before the bot goes hunting, and
// a purse too light for the sword learns its price on the way.
func TestAdventurer_repairsFirst(t *testing.T) {
	addr, store := startGameStore(t, 10*time.Millisecond)
	createCharacter(t, addr, "Wren", "correcthorse")
	setRecord(t, store, "Wren", func(r *player.Record) {
		r.Coins = 50
		for i := range r.Inventory {
			if r.Inventory[i].DefinitionId == "training_dagger" {
				d := 5
				r.Inventory[i].Durability = &d
			}
		}
	})
	a := runAdventurer(t, addr, "Wren")

	ok := assert.Eventually(t, func() bool { return a.Stats().Kills >= 1 }, 60*time.Second, 50*time.Millisecond)
	st := a.Stats()
	require.True(t, ok, "stats: %+v", st)
	assert.Equal(t, 1, st.Repairs)
	assert.Zero(t, st.Bought)
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./bot -run 'TestWantsRepair|TestAdventurer_buysItsKit|TestAdventurer_repairsFirst'`
Expected: build failure on `wantsRepair`; once that builds, the game tests fail on
`Bought`/`Repairs` being zero.

- [ ] **Step 3: Walk through, then the user writes it**

`bot/adventurer.go` -- `town` reads, maybe outfits, and hands the rest to `setOut`:

```go
// town rests if it must, sees to its kit if there's something to see to, then
// sets out.
func (a *Adventurer) town(ctx context.Context) (stateFn, error) {
	text, m, err := a.ask("equipment", powerRe)
	if err != nil {
		return nil, err
	}
	if a.needsRest() {
		// ... as before
	}
	rows := readEquipment(text)
	inv, _, err := a.ask("inventory", inventoryRe)
	if err != nil {
		return nil, err
	}
	coins := readPurse(inv)
	if wantsRepair(rows, coins) || a.affordable(rows, coins) != nil {
		return a.outfit(rows, coins), nil
	}
	power, _ := strconv.Atoi(m[1])
	return a.setOut(ctx, power)
}

// setOut picks a ground its power suits, or waits in the market for one.
func (a *Adventurer) setOut(ctx context.Context, power int) (stateFn, error) {
	a.count(func(st *Stats) { st.Power = power })
	a.ground = a.pickGround(power)
	// ... the rest of the old town, from "if a.ground == nil" on
}
```

`bot/outfit.go`:

```go
// wantsRepair: something worn is at the mark, and there's a coin to pay with.
func wantsRepair(rows []worn, coins int) bool {
	if coins == 0 {
		return false
	}
	for _, r := range rows {
		if r.needsRepair() {
			return true
		}
	}
	return false
}

// outfit is a visit to the smith and the store, both off Market Square, and
// then out: once a town visit, never back to town, so a repair it can't
// finish doesn't send it round again.
func (a *Adventurer) outfit(rows []worn, coins int) stateFn {
	return func(ctx context.Context) (stateFn, error) {
		if a.attacked {
			return a.fightThen(a.town), nil
		}
		if a.here == home {
			if _, err := a.walk(ctx, waitIn); err != nil {
				return nil, err
			}
		}
		if wantsRepair(rows, coins) {
			if _, err := a.walk(ctx, step{"west", "Smithy"}); err != nil {
				return nil, err
			}
			if _, _, err := a.ask("repair all", repairRe); err != nil {
				return nil, err
			}
			a.count(func(st *Stats) { st.Repairs++ })
			if _, err := a.walk(ctx, step{"east", "Market Square"}); err != nil {
				return nil, err
			}
			inv, _, err := a.ask("inventory", inventoryRe)
			if err != nil {
				return nil, err
			}
			coins = readPurse(inv)
		}
		if a.affordable(rows, coins) != nil {
			if _, err := a.walk(ctx, step{"east", "General Store"}); err != nil {
				return nil, err
			}
			if err := a.shop(ctx, rows, coins); err != nil {
				return nil, err
			}
			if _, err := a.walk(ctx, step{"west", "Market Square"}); err != nil {
				return nil, err
			}
		}
		_, m, err := a.ask("equipment", powerRe)
		if err != nil {
			return nil, err
		}
		power, _ := strconv.Atoi(m[1])
		return a.setOut(ctx, power)
	}
}
```

Walkthrough points: why `outfit` ends in `setOut` and not `town` (Review Focus 1); why
the purse is read again after the smith (a repair's cost isn't worth adding up from its
lines when the inventory says it); why `a.here` only needs the `home` case (`town` is
reached from recall and the donation room, both ending in Temple Square, or from the
market); why `setOut` records `Power` (the game tests read it, and it's the number the
bot itself acts on).

- [ ] **Step 4: Run the whole bot package**

Run: `go test ./bot`
Expected: PASS -- the new tests, and every pipe test that drives `town` (Review Focus 5).

- [ ] **Step 5: Review, `make check`, commit**

```bash
git add bot/adventurer.go bot/outfit.go bot/outfit_test.go bot/outfit_game_test.go
git commit -m "Bots: hunters see the smith and the store on their way out of town

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Giving away the starter pieces

**Files:**
- Modify: `bot/adventurer.go` (`dropRe`, `donate`)
- Test: `bot/outfit_test.go`

**Interfaces:**
- Consumes: `spares()` (Task 1)

- [ ] **Step 1: Write the failing test (Claude)**

```go
// The donation trip drops the ground's loot and then the starter pieces --
// one still worn is an answer, not a lost bot.
func TestDonate_givesAwayTheStarterPieces(t *testing.T) {
	a, g := adventurer(t)
	a.ground = &grounds[0]
	done := make(chan error, 1)
	go func() { _, err := a.donate(context.Background()); done <- err }()

	assert.Equal(t, "recall", g.heard())
	g.say("Temple Square\r\n A square.\r\n[ Exits: e s ]\r\n<100/100hp> ")
	assert.Equal(t, "east", g.heard())
	g.say("Donation Room\r\n A room.\r\n[ Exits: w ]\r\n<100/100hp> ")
	for _, kw := range a.ground.loot {
		assert.Equal(t, "drop all."+kw, g.heard())
		g.say("You aren't carrying that.\r\n<100/100hp> ")
	}
	assert.Equal(t, "drop all.training", g.heard())
	g.say("You'll have to remove it first.\r\n<100/100hp> ")
	assert.Equal(t, "drop all.wool", g.heard())
	g.say("Dropped.\r\n<100/100hp> ")
	assert.Equal(t, "drop all.travelers", g.heard())
	g.say("You aren't carrying that.\r\n<100/100hp> ")
	assert.Equal(t, "west", g.heard())
	g.say("Temple Square\r\n A square.\r\n[ Exits: e s ]\r\n<100/100hp> ")
	require.NoError(t, <-done)
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./bot -run TestDonate_givesAwayTheStarterPieces`
Expected: FAIL -- the bot says `west` where the test wants `drop all.training`.

- [ ] **Step 3: Walk through, then the user writes it**

```go
	dropRe = regexp.MustCompile(`(?m)^(?:Dropped\.|You aren't carrying that\.|You'll have to remove it first\.)$`)
```

and in `donate`:

```go
	// what it found, and what it took off for what it bought
	for _, kw := range append(slices.Clone(a.ground.loot), spares()...) {
```

Walkthrough points: why always all three spares rather than remembering what it took
off (a reconnect forgets, and a "You aren't carrying that." costs one line); why
`drop` never takes a bought piece (worn things stay on, and no spare alias is on a kit
piece -- `TestKit_theStoreSellsIt` holds that).

- [ ] **Step 4: Run the bot package**

Run: `go test ./bot`
Expected: PASS.

- [ ] **Step 5: Review, `make check`, commit**

```bash
git add bot/adventurer.go bot/outfit_test.go
git commit -m "Bots: what a hunter took off goes to the donation room

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Docs (Claude)

**Files:**
- Modify: `CLAUDE.md` (bot/ section, after the hunter paragraph)
- Modify: `telnet/help.go` (`help bots`)
- Modify: `docs/working.md` ("On master, not yet released"), `ROADMAP.md` (bots line)
- Modify: the spec's status line

- [ ] **Step 1: CLAUDE.md** -- a paragraph after the hunting-grounds one:

```markdown
**Hunters buy their kit** (`bot/outfit.go`): `town` reads the equipment rows and the
purse, and when something worn is at the repair mark (broken, or a third left -- the
yellow) with a coin to pay, or the next piece of `kit` is affordable, goes on an
`outfit` visit off Market Square -- `repair all` at the Smithy, then `buy`, `remove`
the starter piece, `wear`, down the kit while the purse lasts -- and then out, never
back to `town`. Prices are learned from the store's answers, in memory. The kit is the
General Store's power-4 pieces, which with the token make power 3: still a farm
hunter. The donation trip drops the starter pieces by alias (`spares`).
`TestKit_theStoreSellsIt` holds the kit to the real store and starting gear.
```

and the "a bot is power 1 for good (it never wears loot)" sentence becomes "a bot
never wears loot, and the kit it buys tops out at power 3".

- [ ] **Step 2: `help bots`** -- after "give what they find to the donation room, east
of Temple Square;": "they spend their coins on gear from the General Store and on the
smith;". Run `go test ./telnet` (help's tests read the text).

- [ ] **Step 3: working.md and ROADMAP.md** -- under "On master, not yet released":
"Hunter bots buy the General Store's power-4 kit and keep it repaired; the store sells
it, and a strong draught, to players too." The site guide's store line grows with the
release. In ROADMAP, the bots-that-grow item notes option 1 done (2026-10-15).

- [ ] **Step 4: the spec's status** -- "**built 2026-10-15**".

- [ ] **Step 5: `make check`, commit**

```bash
git add CLAUDE.md telnet/help.go docs/working.md ROADMAP.md docs/superpowers/specs/2026-10-15-bots-buy-their-kit-design.md
git commit -m "Docs: hunters buy their kit

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
