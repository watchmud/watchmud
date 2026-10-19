# Hunters climb the Hollowfields -- Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Hand-implemented, with the usual split:** Claude writes the tests and the docs.
> The user writes the Go production code in `bot/`, and Claude walks through each
> change first -- what it does, why it's shaped that way -- then reviews it and runs
> `make check` before the commit. Walkthrough code never says "unchanged"; it shows
> the whole kept block.

**Goal:** A hunter in the full store kit (power 3) leaves the farms to new players and
hunts the bandit track and the boar wood, wears a find of higher power than what's in
its slot, donates the rest, and has its finds mended at the smith.

**Architecture:** Two more entries in `grounds` (`bot/grounds.go`), and the farms'
`maxPower` down to 2. A `finds` table beside `kit` (`bot/outfit.go`) names the
wearables those grounds drop; `readInventory` reads the pack; `upgrades` decides, from
the equipment rows and the pack, which find goes on in which slot. The donation trip
(`donate`) does the swapping in the Donation Room before it drops anything, using
`look <kw>` to be sure the copy it wears is the best one it carries.

**Tech Stack:** Go, testify, the bot's telnet client, the real content in-process.

**Spec:** `docs/superpowers/specs/2026-10-18-hunters-climb-design.md`

## Global Constraints

- `make check` green before every commit; one commit per task, on master.
- The bot imports nothing of the server and reads rendered text. Every pattern is
  anchored to a line start (`(?m)^`). Fix a bot pattern, never the renderer.
- Farm grounds: `minPower` 0, `maxPower` 2. New grounds: `minPower` 3, `maxPower` 5.
- No route or patrol goes south from The Edge of the Old Wood (wolves) or south from
  The Woodcutter's Clearing (the witch).
- Prey keywords: "lookout" (bandit lookout), "thug" (bandit -- the lookout answers to
  "bandit" too), "boar" (wild boar).
- Find keywords: "hood", "cudgel", "boar" (never "jerkin": the quilted jerkin answers
  to it).
- Wear a find only at **strictly** higher power than the slot's row; an empty slot or
  a broken row counts as 0. Never cast.
- `keepOut`, `consider`'s rule and `TestKeepOut_safe` unchanged.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Two of the same find in the pack.** Inventory is oldest first and `wear hood`
   takes the first hood, so the bot could put on a lesser copy -- or, having just
   removed its old hood, put that straight back on. `wearBest` looks before it wears
   and drops lesser copies until the first one is the best. Pinned in Task 3
   (`TestDonate_wearsTheBestCopy`).
2. **A slot held by something it can't name.** If `repairWord` has no word for the
   occupant it can't `remove` it, so it mustn't buy into it either: `upgrades` skips
   that slot. Pinned in Task 2 (`TestUpgrades`).
3. **A power-4 bot and the wolves.** Growth to 4 makes a power-5 wolf "fair". Only the
   routes keep it out; `TestGrounds_safe` fails on any route or patrol room an
   aggressive mob above `minPower+1` can be in. Task 1.
4. **The kit piece it took off.** A removed studded cap is no starter spare and not
   ground loot; unless it's dropped too it rides around forever. `donate` drops what
   `wearBest` removed. Pinned in Task 3.
5. **A farm bot's donation trip.** Nothing to swap must still be two reads and the
   same drops as today, not an error. Pinned in Task 3 (the updated
   `TestDonate_givesAwayTheStarterPieces`).

---

### Task 1: The bandit track, the boar wood, and the farms left to new players

**Files:**
- Modify: `bot/grounds.go` (the two new `ground`s; farms' `maxPower`; the comment above `hollowfieldsRoute`)
- Test: `bot/grounds_test.go`, `bot/adventurer_test.go` (`TestPickGround_spreadsOut`)

**Interfaces:**
- Produces: `grounds[2]` named `"the bandit track"`, `grounds[3]` named `"the boar wood"`.

- [ ] **Step 1: Write the failing tests** (Claude)

In `bot/grounds_test.go`:

```go
// No ground walks a bot past something that would attack it and is more
// than a power above the weakest bot that hunts there -- measured from 1 for
// the farms, a new character's power, as TestKeepOut_safe does: the
// moonstruck wild dogs (2) roam the whole zone.
func TestGrounds_safe(t *testing.T) // builds the same danger map TestKeepOut_safe
// does (aggressive or moonstruck; a wanderer is in every room of its zone),
// for power above max(g.minPower, 1)+1, and asserts no route or patrol room of g
// is in it, by room name.

// Every prey's room line is a real mob's, of that name, in the Hollowfields.
func TestGrounds_preyIsReal(t *testing.T) // loader.LoadContent("../content");
// for each ground and prey: some MobileDefinition with Name == p.name has
// DescriptionInRoom == p.seen and p.keyword among its aliases.

// The farms are for new characters; a bot in the full kit climbs.
func TestGrounds_bands(t *testing.T) {
	for _, g := range grounds[:2] {
		assert.Equal(t, 2, g.maxPower, g.name)
	}
	for _, g := range grounds[2:] {
		assert.Equal(t, 3, g.minPower, g.name)
	}
}
```

The danger-map half of `TestKeepOut_safe` moves into a helper both tests call:
`func danger(t *testing.T, content *loader.Content, above int) map[*spaces.Room][]string`.

In `bot/adventurer_test.go`, `TestPickGround_spreadsOut` becomes: power 1 picks both
farms and never a new ground; power 3 picks both new grounds and never a farm.
`TestGrounds_walk` needs nothing: it walks every ground, and its homeward from every
patrol room.

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./bot -run 'TestGrounds_|TestPickGround'`
Expected: FAIL -- `grounds[2:]` is empty, farms' `maxPower` is 5.

- [ ] **Step 3: The grounds** (user)

In `bot/grounds.go`, farms' `maxPower: 2`, and:

- `"the bandit track"`: route `hollowfieldsRoute`; patrol east Along the Hedgerow,
  south The Wheat Field, south A Narrow Track, east The Bandit Camp, west A Narrow
  Track, north The Wheat Field, west Farm Lane, north The Waystone. Prey
  `{"lookout", "bandit lookout", "A bandit lookout leans on a stick, pretending not to watch the path."}`,
  `{"thug", "bandit", "A bandit in a patched jerkin fingers a cudgel."}`. Loot
  `"hood", "cudgel"`.
- `"the boar wood"`: route `hollowfieldsRoute`; patrol south Farm Lane, south The Old
  Bridge, south The Edge of the Old Wood, west The Woodcutter's Clearing, east The
  Edge of the Old Wood, north The Old Bridge, north Farm Lane, north The Waystone.
  Prey `{"boar", "wild boar", "A wild boar roots at the ground, tusks muddy."}`. Loot
  `"boar"`.

The comment above `hollowfieldsRoute` says why the farms stop at 2 and why the
Old Wood stops at its edge.

- [ ] **Step 4: Run the bot tests**

Run: `go test ./bot`
Expected: PASS, including `TestGrounds_walk` for both new grounds.

- [ ] **Step 5: Commit** -- "Bots: the bandit track and the boar wood, and the farms left to new players"

---

### Task 2: What the grounds drop, read off the pack

**Files:**
- Modify: `bot/outfit.go` (`finds`, `carried`, `readInventory`, `repairWord`, `swap`, `upgrades`)
- Test: `bot/outfit_test.go`, `bot/outfit_game_test.go`

**Interfaces:**
- Consumes: `worn`, `readEquipment`, `kit` (existing).
- Produces:
  - `type find struct{ item, keyword, slot string }` -- `item` is the short
    description, exactly as `equipment` and `inventory` show it.
  - `var finds = []find{{"a leather bandit hood", "hood", "head"}, {"a knotted cudgel", "cudgel", "wielded"}, {"a boar-hide jerkin", "boar", "body"}}`
  - `type carried struct{ item string; power, count int; broken bool }`
  - `func readInventory(text string) []carried` -- a folded line `(x3)` is one entry
    with `count` 3, otherwise 1; the purse line and the heading aren't items.
  - `func (w worn) repairWord() string` -- also a find's keyword when `w.item` is
    that find's.
  - `type swap struct{ on find; power, copies int; off string }` -- `power` is the
    best copy's, `copies` how many unbroken copies of the find the pack holds (the
    folded `(xN)` counts N, so `carried` keeps a `count`), `off` the word that
    removes the slot's occupant, `""` for an empty slot.
  - `func upgrades(rows []worn, pack []carried) []swap` -- in `finds` order, at most
    one per slot: the highest-power unbroken find carried for it, if strictly above
    the slot's row (empty or broken is 0); none for a slot whose occupant has no
    `repairWord`.

- [ ] **Step 1: Write the failing tests** (Claude)

```go
func TestReadInventory(t *testing.T) {
	inv := readInventory("Inventory\n" +
		"  a leather bandit hood (x2)  power 4  20/20\n" +
		"  a knotted cudgel            power 5  25/25\n" +
		"  a studded cap               power 4  broken\n" +
		"  a long goose feather        power 1\n" +
		"You have 12 coins.\n")
	assert.Equal(t, []carried{
		{item: "a leather bandit hood", power: 4, count: 2},
		{item: "a knotted cudgel", power: 5, count: 1},
		{item: "a studded cap", power: 4, count: 1, broken: true},
		{item: "a long goose feather", power: 1, count: 1},
	}, inv)
	assert.Empty(t, readInventory("You aren't carrying anything.\nYou have 3 coins.\n"))
}

func TestRepairWord_finds(t *testing.T) {
	assert.Equal(t, "hood", worn{slot: "head", item: "a leather bandit hood"}.repairWord())
	assert.Equal(t, "boar", worn{slot: "body", item: "a boar-hide jerkin"}.repairWord())
}

func TestUpgrades(t *testing.T) {
	kitOn := []worn{
		{slot: "wielded", item: "an arming sword", power: 4},
		{slot: "body", item: "a quilted jerkin", power: 4},
		{slot: "head", item: "a studded cap", power: 4},
	}
	pack := []carried{
		{item: "a leather bandit hood", power: 4, count: 2},
		{item: "a leather bandit hood", power: 5, count: 1},
		{item: "a knotted cudgel", power: 4, count: 1},
		{item: "a boar-hide jerkin", power: 6, count: 1, broken: true},
	}
	assert.Equal(t, []swap{{on: finds[0], power: 5, copies: 3, off: "studded"}}, upgrades(kitOn, pack),
		"the better hood only: an equal cudgel stays in the pack, a broken jerkin counts for nothing")

	oneHood := []carried{{item: "a leather bandit hood", power: 4, count: 1}}
	assert.Equal(t, []swap{{on: finds[0], power: 4, copies: 1, off: ""}}, upgrades(nil, oneHood),
		"an empty slot is power 0")
	brokenCap := []worn{{slot: "head", item: "a studded cap", power: 4, broken: true}}
	assert.Equal(t, []swap{{on: finds[0], power: 4, copies: 1, off: "studded"}}, upgrades(brokenCap, oneHood),
		"a broken row counts for nothing")
	strange := []worn{{slot: "head", item: "a jester's cap", power: 1}}
	assert.Empty(t, upgrades(strange, pack), "it can't take off what it can't name")
}
```

In `bot/outfit_game_test.go`, `TestFinds_theContentHasThem`: against the real content,
each find is a Hollowfields object with that `ShortDescription`, worn in that slot
(the slot label as `equipment` prints it), with `keyword` among its aliases -- and
no kit piece, store stock or starting-gear object answers to `keyword`.

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./bot -run 'TestReadInventory|TestRepairWord|TestUpgrades|TestFinds_'`
Expected: FAIL to compile -- `readInventory`, `carried`, `finds`, `swap`, `upgrades` undefined.

- [ ] **Step 3: The table and the two readers** (user)

`readInventory` gets its own row pattern: two spaces in, the description (with an
optional ` (xN)` taken off), `power N`, an optional condition. `upgrades` builds
the best carried, unbroken find per slot first, then compares with the rows.

- [ ] **Step 4: Run the bot tests**

Run: `go test ./bot`
Expected: PASS. `TestRepairWord`'s "not the kit's" assertion on `"a bandit hood"`
still holds: that's not a find's short description.

- [ ] **Step 5: Commit** -- "Bots: what the bandits and boars drop, and what's worth wearing"

---

### Task 3: Wearing the better find on the donation trip

**Files:**
- Modify: `bot/adventurer.go` (`donate`, `Stats`, `setOut`), `bot/outfit.go` (`wearBest`, `lookedRe`)
- Modify: `CLAUDE.md` (the hunter and "Hunters buy their kit" paragraphs), `ROADMAP.md` (the bots line), `docs/working.md` ("On master, not yet released")
- Test: `bot/outfit_test.go`, `bot/outfit_game_test.go`

**Interfaces:**
- Consumes: `readInventory`, `readEquipment`, `upgrades`, `swap` (Task 2); `grounds[2:]` (Task 1).
- Produces:
  - `Stats.Upgrades int` (finds worn) and `Stats.Ground string` (the ground it last set out for).
  - `var lookedRe = regexp.MustCompile(`(?m)^  (?:.+\. )?Power (\d+)\.$`)` -- the power line of `look <thing>`.
  - `func (a *Adventurer) wearBest(ctx context.Context, sw swap) (removed string, err error)`.

- [ ] **Step 1: Write the failing tests** (Claude)

`TestDonate_givesAwayTheStarterPieces` grows two reads after "Donation Room":
`inventory` (answered with the waterskin and a purse) and `equipment` (the starter
equipment), then the same drops as today.

```go
// In the donation room, before it gives anything away: the better hood goes
// on, the cap it replaces goes with the rest. Two hoods in the pack, the
// lesser first: it looks before it wears, and gives the lesser away.
func TestDonate_wearsTheBestCopy(t *testing.T) {
	a, g := adventurer(t)
	a.ground = &grounds[2]
	done := make(chan error, 1)
	go func() { _, err := a.donate(context.Background()); done <- err }()

	assert.Equal(t, "recall", g.heard())
	g.say("Temple Square\r\n A square.\r\n[ Exits: e s ]\r\n<100/100hp> ")
	assert.Equal(t, "east", g.heard())
	g.say("Donation Room\r\n A room.\r\n[ Exits: w ]\r\n<100/100hp> ")
	assert.Equal(t, "inventory", g.heard())
	g.say("Inventory\r\n  a leather bandit hood  power 4  20/20\r\n  a leather bandit hood  power 5  20/20\r\nYou have 9 coins.\r\n<100/100hp> ")
	assert.Equal(t, "equipment", g.heard())
	g.say("Equipment (power 3)\r\n  head  a studded cap  power 4  20/20\r\n  neck  a temple token  power 1\r\n<100/100hp> ")
	assert.Equal(t, "remove studded", g.heard())
	g.say("You stop using a studded cap.\r\n<100/100hp> ")
	assert.Equal(t, "look hood", g.heard())
	g.say("A leather bandit hood\r\n  Leather armor, worn on the head. Power 4.\r\n<100/100hp> ")
	assert.Equal(t, "drop hood", g.heard(), "the lesser copy, first in the pack")
	g.say("Dropped.\r\n<100/100hp> ")
	assert.Equal(t, "look hood", g.heard())
	g.say("A leather bandit hood\r\n  Leather armor, worn on the head. Power 5.\r\n<100/100hp> ")
	assert.Equal(t, "wear hood", g.heard())
	g.say("You wear a leather bandit hood.\r\n<100/100hp> ")
	for _, kw := range []string{"hood", "cudgel", "training", "wool", "travelers", "studded"} {
		assert.Equal(t, "drop all."+kw, g.heard())
		g.say("You aren't carrying that.\r\n<100/100hp> ")
	}
	assert.Equal(t, "west", g.heard())
	g.say("Temple Square\r\n A square.\r\n[ Exits: e s ]\r\n<100/100hp> ")
	require.NoError(t, <-done)
	assert.Equal(t, 1, a.Stats().Upgrades)
}
```

In `bot/outfit_game_test.go`, `TestAdventurer_climbs`: a character set up with 2000
coins buys the whole kit, reaches power 3, and kills something on one of the new
grounds -- `assert.Eventually` on `Stats().Kills >= 1`, then `Stats().Power == 3` and
`Stats().Ground` one of `"the bandit track"`, `"the boar wood"`.

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./bot -run 'TestDonate_|TestAdventurer_climbs'`
Expected: FAIL -- `donate` drops straight away; `Stats` has no `Upgrades` or `Ground`.

- [ ] **Step 3: `wearBest` and the donation trip** (user)

`wearBest`: remove `sw.off` if there is one (`removedRe`). Then at most
`sw.copies + 1` times (the +1 is the piece just removed, when it was a copy too):
`look <kw>` (`lookedRe`); at `sw.power`,
`wear <kw>` (`woreRe`) and count `Upgrades`; below it, `drop <kw>` (`dropRe`) and
look again. It returns `sw.off`.

`donate`, once in the Donation Room: `inventory`, `equipment`, `wearBest` for each of
`upgrades(rows, pack)`, then `drop all.<kw>` for the ground's loot, `spares()`, and
each word `wearBest` removed -- in that order, without repeats. `setOut` records
`Ground`.

- [ ] **Step 4: Docs** (Claude)

CLAUDE.md: the bot paragraph's "its grounds are the farms" becomes the four grounds
by power, and "Hunters buy their kit" gains what a hunter does with finds (where it
decides, the look-before-wear rule, mending by find keyword). ROADMAP's bots line:
option 2 done, with the date. `docs/working.md`: a line under "On master, not yet
released" (for the site: nothing).

- [ ] **Step 5: Run everything**

Run: `make check`
Expected: green.

- [ ] **Step 6: Commit** -- "Bots: a hunter wears the better find and gives the rest away"
