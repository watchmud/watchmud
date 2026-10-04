# The Barrow-King's guard: `me:summon` -- Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **This one is hand-implemented, with a split:** Claude writes the tests, the Lua, the
> test content and the docs. The user writes the Go production code, and Claude walks
> through each change first -- what it does, why it's shaped that way, what it touches --
> then reviews it before the commit. The Go below is the reference the walkthrough
> explains, not something to paste.

**Goal:** At half health the Barrow-King says his line and two barrow skeletons rise,
each going for a player in the room; they crumble to dust when his fight is over, and
nothing about them can be farmed or block a respawn.

**Architecture:** Content declares what a mob may summon (`"summons"` in mobs.json,
resolved by the loader onto `mobile.Definition.Summons`). The script runtime gets
`me:summon(id, count)` and `me.summons`, checks the id against that list, applies the
caps, and calls the world through a `script.Actions` struct that replaces the lone `Say`
callback. `world/summons.go` owns a summon's life: arriving (`summon`), picking a
target, and crumbling -- on the summoner's death in `combatantDied`, and by a sweep at
the end of `DoViolence` for every other way a fight ends.

**Tech Stack:** Go, gopher-lua, testify (`assert`/`require`/`suite`), testdice.

**Spec:** `docs/superpowers/specs/2026-10-04-king-summons-design.md`

## Global Constraints

- `make check` (fmt-check, vet, test) green before every commit; one commit per task, on master.
- `script.MaxSummonsPerCall = 4` (across all `me:summon` calls in one hook call);
  `script.MaxLiveSummons = 4` (alive at once, per summoner). Over either: fewer arrive,
  no error. `count < 1` and an id not in the mob's `"summons"` are script errors.
- `"summons"` resolves like loot: bare is the mob's own zone, `"zone/id"` any other;
  anything unresolved fails startup.
- A summon: no corpse, no loot, no coins; not counted by `Occupancy.MobileCount`;
  crumbles when its summoner dies or its summoner's fight ends.
- Each summon fights a player in the room picked through `w.roller`, skipping a wizard
  with `nohassle`; nobody to pick, it stands.
- The King: summons `barrow_skeleton` ×2 once per fight at `health <= max_health / 2`,
  with the say `"Rise, my guard! Rise and defend your king!"`; `on_fight_start` resets it.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Two refinements of the spec, decided while planning

1. **`Summoner *mobile.Instance`, not a `SummonedBy` uuid.** The sweep has to ask "is
   my summoner still in the world, and fighting?", and with a pointer that is
   `w.roomOf(mob.Summoner) == nil || !InFight(mob.Summoner)` -- no lookup by id, which
   nothing in the world has. A removed mob is out of `Occupancy`, so a stale pointer
   answers "gone" correctly. Mobs are never saved, so nothing has to serialize it.
2. **A fighter who has left the world doesn't swing.** `DoViolence` ranges over a
   snapshot of the fights. If the King dies to a blow early in a round, his summons
   crumble -- but their fights are still in the snapshot, and a crumbled skeleton isn't
   `Dead()`, so it would swing from nowhere (no room to tell, damage still dealt). One
   guard -- skip a fighter `roomOf` can't place -- closes it for crumbles and anything
   else that removes a mob mid-round.

## Review Focus

1. **The King dies mid-round and a summon's fight is later in the same round's
   snapshot** -- expected: the crumbled summon never swings. Pinned in Task 3
   (`TestRemovedFighterDoesntSwing`).
2. **A summon is killed while its summoner fights on** -- expected: `Died`, then
   crumble; no corpse; the summoner's other summons stay. Pinned in Task 3.
3. **A group wipes and walks back in** -- expected: the summons crumbled with the
   fight, and the King summons again in the new one. Pinned in Task 3 (sweep) and
   Task 5 (fresh fight re-summons).
4. **A script summons in a loop** -- expected: at most 4 arrive per call and 4 live per
   summoner, no error, no flood. Pinned in Task 4.
5. **The only player in the room is a `nohassle` wizard** -- expected: summons arrive,
   attack nobody, crumble with the fight. Pinned in Task 2.

---

## File map

- `mobile/definition.go` -- `Summons []*Definition`
- `mobile/instance.go` -- `Summoner *Instance`
- `loader/mobfile.go` -- `"summons"` JSON key
- `loader/summons.go` (new) -- `mobileRef`, `mobSummons`
- `loader/content.go` -- second pass in `loadMobileDefinitions`
- `spaces/occupancy.go` -- `MobileCount` skips summons
- `event/events.go` -- `Summoned`, `Crumbled`
- `telnet/render.go` -- their two cases
- `world/summons.go` (new) -- `summon`, `summonAttacks`, `liveSummons`, `crumble`,
  `crumbleSummonsOf`, `sweepSummons`
- `world/violence.go` -- `combatantDied` branches, the removed-fighter guard, the sweep
- `script/runtime.go` -- `Actions`, `me:summon`, `me.summons`, the caps
- `world/world.go` -- `script.NewRuntime(..., script.Actions{...})`
- Content: `content/world/barrow/mobs.json`, `content/world/barrow/scripts/barrow_king.lua`
- Test content: `testcontent/world/wrathrock/{mobs,instructions}.json`,
  `testcontent/world/wrathrock/scripts/warlock.lua`
- Tests: `loader/summons_test.go`, `world/summons_test.go`, `script/runtime_test.go`,
  `world/scripts_test.go`, `world/king_test.go`, `telnet/render_test.go`
- Docs: CLAUDE.md, ROADMAP.md, LEVELS.md

---

### Task 1: Content says what a mob may summon

**Files:**
- Modify: `mobile/definition.go`, `loader/mobfile.go`, `loader/content.go` (`loadMobileDefinitions`)
- Create: `loader/summons.go`, `loader/summons_test.go`
- Modify: `content/world/barrow/mobs.json` (the King's `"summons"`)
- Modify: `testcontent/world/wrathrock/mobs.json` (an `imp`)

**Interfaces:**
- Produces: `mobile.Definition.Summons []*mobile.Definition`;
  `(*loader.Content).mobileRef(zoneName, ref string) (*mobile.Definition, error)`;
  `(*loader.Content).mobSummons(zoneName string, mob mobEntry) ([]*mobile.Definition, error)`.

- [ ] **Step 1 (Claude): write the failing tests** -- `loader/summons_test.go`:

```go
package loader

import (
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/rules"
)

// summonsContent is lootContent's wrathrock and caves, with a mob in each.
func summonsContent(t *testing.T) *Content {
	t.Helper()
	c := lootContent(t)
	c.Zones["caves"].AddMobileDefinition(mobile.NewDefinition("bat", "bat", "caves", nil,
		"a bat", "A bat is here.", 5, rules.WanderDefinition{}, 10, false))
	c.Zones["wrathrock"].AddMobileDefinition(mobile.NewDefinition("guard", "guard", "wrathrock", nil,
		"a guard", "A guard is here.", 20, rules.WanderDefinition{}, 10, false))
	return c
}

// A bare id is the mob's own zone; "zone/id" is anywhere.
func TestMobSummons_resolves(t *testing.T) {
	c := summonsContent(t)
	summons, err := c.mobSummons("caves", mobEntry{Id: "lich", Summons: []string{"bat", "wrathrock/guard"}})
	require.NoError(t, err)
	require.Len(t, summons, 2)
	assert.Same(t, c.Zones["caves"].MobileDefinitions["bat"], summons[0])
	assert.Same(t, c.Zones["wrathrock"].MobileDefinitions["guard"], summons[1])
}

func TestMobSummons_noneIsNone(t *testing.T) {
	summons, err := summonsContent(t).mobSummons("caves", mobEntry{Id: "rat"})
	require.NoError(t, err)
	assert.Empty(t, summons)
}

func TestMobSummons_errors(t *testing.T) {
	c := summonsContent(t)
	for name, ref := range map[string]string{
		"not defined":    "wraith",
		"zone not found": "atlantis/squid",
		"no mob named":   "",
		"zone, no mob":   "caves/",
	} {
		_, err := c.mobSummons("caves", mobEntry{Id: "lich", Summons: []string{ref}})
		assert.ErrorContains(t, err, "caves/lich", name)
	}
}

// Summons name mobs, and mobs load a zone at a time, so a mob may name one in
// a zone that isn't loaded yet: caves loads before wrathrock.
func TestLoadMobileDefinitions_summonsALaterZone(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{
		"caves/mobs.json":     {Data: []byte(`[{"id": "lich", "name": "lich", "max_health": 30, "summons": ["wrathrock/guard"]}]`)},
		"wrathrock/mobs.json": {Data: []byte(`[{"id": "guard", "name": "guard", "max_health": 20}]`)},
	}
	require.NoError(t, c.loadMobileDefinitions(fsys))

	lich := c.Zones["caves"].MobileDefinitions["lich"]
	require.Len(t, lich.Summons, 1)
	assert.Same(t, c.Zones["wrathrock"].MobileDefinitions["guard"], lich.Summons[0])
}

func TestLoadMobileDefinitions_unknownSummonFails(t *testing.T) {
	c := lootContent(t)
	fsys := fstest.MapFS{
		"caves/mobs.json": {Data: []byte(`[{"id": "lich", "name": "lich", "max_health": 30, "summons": ["wraith"]}]`)},
	}
	assert.ErrorContains(t, c.loadMobileDefinitions(fsys), "caves/lich")
}

// and through a real load: the King calls up barrow skeletons
func TestLoadContent_summons(t *testing.T) {
	c, err := LoadContent(os.DirFS("../content"))
	require.NoError(t, err)
	king := c.Zones["barrow"].MobileDefinitions["barrow_king"]
	require.Len(t, king.Summons, 1)
	assert.Same(t, c.Zones["barrow"].MobileDefinitions["barrow_skeleton"], king.Summons[0])
}
```

Content (Claude): add `"summons": ["barrow_skeleton"]` to `barrow_king` in
`content/world/barrow/mobs.json`, and an imp to `testcontent/world/wrathrock/mobs.json`
for the later tasks:

```json
{"id":"imp","name":"imp","short_description":"An imp.","description_in_room":"An imp capers here.","aliases":["imp"],"max_health":5,"wandering_definition":{"can_wander":false},"ac":10}
```

- [ ] **Step 2: run them, see them fail** -- `go test ./loader -run 'Summons|summon'`.
  Expected: build failure, `mobEntry` has no field `Summons`.

- [ ] **Step 3 (user, walked through): the Go.**

`mobile/definition.go`, beside `Loot` and `Script`:

```go
	// Summons is every mob this one's script may call up with me:summon.
	// Content, resolved at load like Loot, so a script can't name the wrong
	// thing and a reviewer can see everything a boss can bring in.
	Summons []*Definition
```

`loader/mobfile.go`, in `mobEntry`:

```go
	// Summons names the mobs this one's script may summon: a bare id is this
	// zone's, "zone/id" any other's. See summons.go.
	Summons []string `json:"summons"`
```

`loader/summons.go`:

```go
package loader

import (
	"fmt"
	"strings"

	"github.com/watchmud/watchmud/mobile"
)

// mobSummons resolves what a mob may summon against the mob definitions.
// Anything that doesn't resolve fails startup: a typo would otherwise be a
// summon that fails every time the script reaches for it, in the middle of
// the fight it was written for.
func (c *Content) mobSummons(zoneName string, mob mobEntry) ([]*mobile.Definition, error) {
	var summons []*mobile.Definition
	for _, ref := range mob.Summons {
		defn, err := c.mobileRef(zoneName, ref)
		if err != nil {
			return nil, fmt.Errorf("mob %s/%s: summons: %w", zoneName, mob.Id, err)
		}
		summons = append(summons, defn)
	}
	return summons, nil
}

// mobileRef finds the mob a piece of content names: "zone/id", or a bare id
// for the naming zone's own. objectRef's twin -- but mobs load a zone at a
// time, so only ask once every zone's are in.
func (c *Content) mobileRef(zoneName, ref string) (*mobile.Definition, error) {
	zoneId, mobId := zoneName, ref
	if z, m, found := strings.Cut(ref, "/"); found {
		zoneId, mobId = z, m
	}
	if mobId == "" {
		return nil, fmt.Errorf("names no mob")
	}
	zone, ok := c.Zones[zoneId]
	if !ok {
		return nil, fmt.Errorf("%q: zone not found", ref)
	}
	defn, ok := zone.MobileDefinitions[mobId]
	if !ok {
		return nil, fmt.Errorf("%q: mob not defined", ref)
	}
	return defn, nil
}
```

`loader/content.go`, `loadMobileDefinitions` -- remember who summons during the zone
loop, resolve after it:

```go
func (c *Content) loadMobileDefinitions(fsys fs.FS) error {
	// Summons name mobs, which may be in a zone not loaded yet: held for a
	// second pass, like room exits.
	type summoner struct {
		zone  string
		entry mobEntry
		defn  *mobile.Definition
	}
	var summoners []summoner

	for _, zonename := range c.zoneNames() {
		// ... unchanged, down to AddMobileDefinition ...
			c.Zones[zonename].AddMobileDefinition(defn)
			if len(mob.Summons) > 0 {
				summoners = append(summoners, summoner{zonename, mob, defn})
			}
		}
	}

	for _, s := range summoners {
		summons, err := c.mobSummons(s.zone, s.entry)
		if err != nil {
			return err
		}
		s.defn.Summons = summons
	}
	return nil
}
```

Walkthrough points: why a second pass (the same reason exits are two-pass); why
`mobileRef` mirrors `objectRef` instead of sharing it (different maps, different
types -- a generic helper would cost more to read than the 15 lines it saves); why
`Same` in the tests (it must be *the* definition the zone holds, not a copy).

- [ ] **Step 4: run** `go test ./loader` -- PASS; then `make check`.
- [ ] **Step 5: commit** -- `loader: "summons" -- what a mob's script may call up, resolved at load`

---

### Task 2: A summon arrives, and goes for someone

**Files:**
- Modify: `mobile/instance.go`, `spaces/occupancy.go`, `event/events.go`, `telnet/render.go`
- Create: `world/summons.go`, `world/summons_test.go`
- Test: `telnet/render_test.go`

**Interfaces:**
- Consumes: `mobile.Definition` (Task 1 not required, but the testcontent imp is).
- Produces: `mobile.Instance.Summoner *mobile.Instance`;
  `event.Summoned{Summoner, Name string; Count int}`;
  `(*World).summon(summoner *mobile.Instance, def *mobile.Definition, count int) int`;
  `(*World).liveSummons(summoner *mobile.Instance) int`.

- [ ] **Step 1 (Claude): the failing tests** -- `world/summons_test.go`:

```go
package world

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/testdice"
)

// The Target Drone stands in for a summoner, calling up testcontent's imps in
// temple square, where the player and a second player are standing.
type summonsSuite struct {
	worldTestSuite
	drone *mobile.Instance
	imp   *mobile.Definition
	other *player.Player
	dice  *testdice.LoadedDice
}

func TestSummonsSuite(t *testing.T) {
	suite.Run(t, new(summonsSuite))
}

func (s *summonsSuite) SetupTest() {
	s.worldTestSuite.SetupTest()
	var found bool
	s.drone, found = s.w.StartRoom.FindMobile("target")
	s.Require().True(found)
	s.imp = s.w.Zone("wrathrock").MobileDefinitions["imp"]
	s.Require().NotNil(s.imp)
	s.other = player.NewTestPlayer(uuid.New(), "otherdood", &player.Recorder{})
	s.w.PlacePlayer(s.other, s.w.StartRoom)
	s.dice = testdice.New()
	s.w.roller = s.dice
	s.r.Clear()
}

// imps is every imp in temple square, in the order they arrived.
func (s *summonsSuite) imps() []*mobile.Instance {
	var imps []*mobile.Instance
	for _, m := range s.w.StartRoom.Mobiles() {
		if m.Definition == s.imp {
			imps = append(imps, m)
		}
	}
	return imps
}

func (s *summonsSuite) TestArrive() {
	s.dice.Load([]int{0, 0})

	got := s.w.summon(s.drone, s.imp, 2)

	s.Assert().Equal(2, got)
	imps := s.imps()
	s.Require().Len(imps, 2)
	for _, imp := range imps {
		s.Assert().Same(s.drone, imp.Summoner)
	}
	s.Assert().Equal(event.Summoned{Summoner: "Target Drone", Name: "imp", Count: 2},
		sent[event.Summoned](s.T(), s.r, 0))
	s.Assert().Equal(2, s.w.liveSummons(s.drone))
}

// each picks a player at random: the dice say otherdood, then testdood
func (s *summonsSuite) TestEachGoesForAPlayer() {
	s.dice.Load([]int{1, 0})

	s.w.summon(s.drone, s.imp, 2)

	imps := s.imps()
	s.Require().Len(imps, 2)
	s.Assert().Same(s.other, s.w.fightLedger.GetFight(imps[0]).Fightee)
	s.Assert().Same(s.p, s.w.fightLedger.GetFight(imps[1]).Fightee)
}

// a wizard with nohassle isn't on the list, so both dice pick otherdood
func (s *summonsSuite) TestSkipsANoHassleWizard() {
	s.p.SetWizard(true)
	s.p.SetNoHassle(true)
	s.dice.Load([]int{0, 0})

	s.w.summon(s.drone, s.imp, 2)

	for _, imp := range s.imps() {
		s.Assert().Same(s.other, s.w.fightLedger.GetFight(imp).Fightee)
	}
}

// Review focus 5: nobody to pick -- they arrive and stand
func (s *summonsSuite) TestNobodyToPick() {
	s.w.RemovePlayer(s.other)
	s.p.SetWizard(true)
	s.p.SetNoHassle(true)

	s.Assert().Equal(2, s.w.summon(s.drone, s.imp, 2))

	for _, imp := range s.imps() {
		s.Assert().False(s.w.fightLedger.InFight(imp))
	}
}

// a summon isn't the zone's: it mustn't keep a reset from refilling a room
func (s *summonsSuite) TestNotCountedForResets() {
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)

	s.Assert().Equal(0, s.w.occupancy.MobileCount("imp"))
}
```

`telnet/render_test.go`, a direct test (a summon isn't a command anyone types):

```go
func TestRender_summoned(t *testing.T) {
	assert.Equal(t, "Barrow-King calls up 2 barrow skeletons!\n",
		plain(render(event.Summoned{Summoner: "Barrow-King", Name: "barrow skeleton", Count: 2}, "testdood")))
	assert.Equal(t, "Warlock calls up one imp!\n",
		plain(render(event.Summoned{Summoner: "Warlock", Name: "imp", Count: 1}, "testdood")))
}
```

- [ ] **Step 2: run, see them fail** -- `go test ./world -run TestSummonsSuite`; build
  failure (`summon` undefined).

- [ ] **Step 3 (user, walked through): the Go.**

`mobile/instance.go`, in `Instance`:

```go
	// Summoner is the mob whose script called this one up; nil for one a zone
	// reset or a wizard put here. A summon lives as long as its summoner's
	// fight -- see world/summons.go.
	Summoner *Instance
```

`spaces/occupancy.go`, `MobileCount`:

```go
	for mob := range o.mobiles.All() {
		// A summon is its summoner's, not the zone's: counting it would keep a
		// reset from refilling the room it was called away from.
		if mob.Definition.Id == defId && mob.Summoner == nil {
			count++
		}
	}
```

`event/events.go`:

```go
// Summoned goes to the room: Summoner called up Count of Name, who arrive
// fighting.
type Summoned struct {
	Summoner string
	Name     string
	Count    int
}
```

`telnet/render.go`:

```go
	case event.Summoned:
		if m.Count == 1 {
			return m.Summoner + " calls up one " + m.Name + "!\n"
		}
		return fmt.Sprintf("%s calls up %d %ss!\n", m.Summoner, m.Count, m.Name)
```

`world/summons.go`:

```go
package world

import (
	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/spaces"
)

// summon calls up count of def into summoner's room, each going for a player
// there, and answers how many came. The caps and the "may it summon that?"
// check are the script runtime's; this is the engine doing what it's asked.
func (w *World) summon(summoner *mobile.Instance, def *mobile.Definition, count int) int {
	room := w.mobileRoom(summoner)
	summons := make([]*mobile.Instance, 0, count)
	for range count {
		mob := mobile.NewInstance(def)
		mob.Summoner = summoner
		w.PlaceMobile(mob, room)
		summons = append(summons, mob)
	}
	// All of them, then the fights: the room reads "the King calls up two
	// skeletons" before it reads anything they do.
	room.Send(event.Summoned{Summoner: summoner.Name(), Name: def.Name, Count: count})
	for _, mob := range summons {
		w.summonAttacks(mob, room)
	}
	return count
}

// summonAttacks sends a summon at a player in the room, picked at random, so
// the guard spreads over the group instead of piling onto the tank. A wizard
// with nohassle isn't picked, as aggro doesn't pick them. With nobody to
// pick, the summon stands; it crumbles with the rest.
func (w *World) summonAttacks(mob *mobile.Instance, room *spaces.Room) {
	var targets []*player.Player
	for _, p := range room.Players() {
		if p.IsWizard() && p.NoHassle() {
			continue
		}
		targets = append(targets, p)
	}
	if len(targets) == 0 {
		return
	}
	i, err := w.roller.IntN(len(targets))
	if err != nil {
		log.Error().Err(err).Msgf("summon: %s picking a target", mob.Definition.Id)
		return
	}
	if err := w.startFight(mob, targets[i]); err != nil {
		log.Error().Err(err).Msgf("summon: %s starting its fight", mob.Definition.Id)
	}
}

// liveSummons is how many of summoner's summons are in the world.
func (w *World) liveSummons(summoner *mobile.Instance) int {
	n := 0
	for _, mob := range w.Mobiles() {
		if mob.Summoner == summoner {
			n++
		}
	}
	return n
}
```

Walkthrough points: why `PlaceMobile` and not touching the room's list (`Occupancy` is
the only writer); why `startFight` (it fires a summon's own `on_fight_start` if it has
a script); why `liveSummons` scans rather than keeping a count (one source of truth --
a counter would have to be decremented on every way a mob leaves the world).

- [ ] **Step 4: run** `go test ./world ./telnet ./spaces` -- PASS; `make check`.
- [ ] **Step 5: commit** -- `world: summon -- mobs called up into a room, each going for a player`

---

### Task 3: A summon crumbles with the fight

**Files:**
- Modify: `event/events.go`, `telnet/render.go`, `world/summons.go`, `world/violence.go`
- Test: `world/summons_test.go`, `telnet/render_test.go`

**Interfaces:**
- Consumes: `summon`, `liveSummons`, `Summoner` (Task 2).
- Produces: `event.Crumbled{Name string}`; `(*World).crumble(mob)`,
  `(*World).crumbleSummonsOf(summoner)`, `(*World).sweepSummons()`.

- [ ] **Step 1 (Claude): the failing tests**, appended to `world/summons_test.go`:

```go
// deaths and crumbles in the order the room heard them
func (s *summonsSuite) endings() []any {
	var out []any
	for _, m := range s.r.Sent {
		switch m.(type) {
		case event.Died, event.Crumbled:
			out = append(out, m)
		}
	}
	return out
}

// the summoner dies: his death, then his guard goes to dust
func (s *summonsSuite) TestSummonerDies() {
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)
	s.r.Clear()
	s.dice.Load([]int{99, 99, 0, 0}) // the drone's loot: no knife, the rope (always) and its bump; then coins

	s.w.combatantDied(s.drone, s.w.StartRoom)

	s.Assert().Empty(s.imps())
	s.Assert().Equal([]any{
		event.Died{Target: "Target Drone"},
		event.Crumbled{Name: "imp"},
		event.Crumbled{Name: "imp"},
	}, s.endings())
	s.Assert().False(s.w.fightLedger.InFight(s.p), "nothing left fighting")
}

// Review focus 2: a summon killed leaves nothing, and the rest stay
func (s *summonsSuite) TestKilledSummonLeavesNothing() {
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)
	first := s.imps()[0]
	s.r.Clear()

	s.w.combatantDied(first, s.w.StartRoom)

	s.Assert().Len(s.imps(), 1, "the other stays")
	s.Assert().Empty(s.w.StartRoom.Inventory.FindAll("corpse"), "no corpse, so no loot or coins")
	s.Assert().Equal([]any{event.Died{Target: "imp"}, event.Crumbled{Name: "imp"}}, s.endings())
}

// Review focus 3: the fight ends some other way -- here, everyone leaves it --
// and the next round's sweep crumbles them
func (s *summonsSuite) TestFightEndsOtherwise() {
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)
	s.w.fightLedger.EndAllFightsWith(s.p.Id())
	s.w.fightLedger.EndAllFightsWith(s.other.Id())

	s.w.DoViolence(10)

	s.Assert().Empty(s.imps())
}

// while the summoner fights, the sweep leaves them be
func (s *summonsSuite) TestSweepLeavesAFightGoingOn() {
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)

	s.w.sweepSummons()

	s.Assert().Len(s.imps(), 2)
}

// a summoner gone from the world without dying -- the sweep catches that too
func (s *summonsSuite) TestSummonerGone() {
	s.Require().NoError(s.w.startFight(s.p, s.drone))
	s.dice.Load([]int{0, 0})
	s.w.summon(s.drone, s.imp, 2)
	s.w.fightLedger.EndAllFightsWith(s.drone.Id())
	s.w.RemoveMobile(s.drone)

	s.w.sweepSummons()

	s.Assert().Empty(s.imps())
}

// Review focus 1: DoViolence ranges over a snapshot of the fights, so one
// whose fighter left the world earlier in the round is still in it. It must
// not swing: no room to tell, and the damage would land from nowhere.
func (s *summonsSuite) TestRemovedFighterDoesntSwing() {
	s.dice.Load([]int{0})
	s.w.summon(s.drone, s.imp, 1)
	imp := s.imps()[0]
	s.w.occupancy.RemoveMobile(imp) // out of the world, its fight left behind
	s.Require().NotNil(s.w.fightLedger.GetFight(imp))
	before := s.p.CurrentHealth() + s.other.CurrentHealth()
	s.dice.Load([]int{20, 4}) // a hit, for 4

	s.w.DoViolence(10)

	s.Assert().Equal(before, s.p.CurrentHealth()+s.other.CurrentHealth())
}
```

`telnet/render_test.go`:

```go
func TestRender_crumbled(t *testing.T) {
	assert.Equal(t, "barrow skeleton crumbles to dust.\n",
		plain(render(event.Crumbled{Name: "barrow skeleton"}, "testdood")))
}
```

Note for the reviewer: the dice in `TestSummonerDies` follow the drone's loot table in
testcontent (knife 50%, rope 100%) and `rollCoins`; running out only logs, so a change
there shows up as log noise, not a failure.

- [ ] **Step 2: run, see them fail** -- `go test ./world -run TestSummonsSuite`.

- [ ] **Step 3 (user, walked through): the Go.**

`event/events.go`:

```go
// Crumbled goes to the room: a summon going back where it came from, at the
// end of its summoner's fight or its own. It leaves nothing behind.
type Crumbled struct {
	Name string
}
```

`telnet/render.go`:

```go
	case event.Crumbled:
		return m.Name + " crumbles to dust.\n"
```

`world/summons.go`, appended:

```go
// crumble takes a summon out of the world: no corpse, no loot, no coins. Its
// fights end the way a death's do, so whoever it was holding retargets.
func (w *World) crumble(mob *mobile.Instance) {
	if room := w.roomOf(mob); room != nil {
		room.Notify(event.Crumbled{Name: mob.Name()})
	}
	w.fightLedger.EndAllFightsWith(mob.Id())
	w.RemoveMobile(mob)
}

// crumbleSummonsOf crumbles everything summoner called up.
func (w *World) crumbleSummonsOf(summoner *mobile.Instance) {
	for _, mob := range w.Mobiles() {
		if mob.Summoner == summoner {
			w.crumble(mob)
		}
	}
}

// sweepSummons crumbles every summon whose summoner's fight is over: the
// summoner gone from the world, or here and fighting nobody. One check, run
// every round, covers every way a fight ends -- fled, wiped, ways not written
// yet. A death doesn't wait for it: combatantDied crumbles them on the spot.
func (w *World) sweepSummons() {
	for _, mob := range w.Mobiles() {
		s := mob.Summoner
		if s == nil {
			continue
		}
		if w.roomOf(s) == nil || !w.fightLedger.InFight(s) {
			w.crumble(mob)
		}
	}
}
```

`world/violence.go` -- three changes.

At the top of the fight loop in `DoViolence`, after the `Dead()` check:

```go
		// The fights are a snapshot: one whose fighter left the world earlier
		// this round -- a summon crumbling with its summoner -- is still in it,
		// and must not swing from nowhere.
		if w.roomOf(fight.Fighter) == nil {
			continue
		}
```

At the end of `DoViolence`, after the loop:

```go
	w.sweepSummons()
```

`combatantDied`:

```go
func (w *World) combatantDied(dead combat.Combatant, room *spaces.Room) {
	// A summon dies like anything else, and then leaves no body.
	if mob, ok := dead.(*mobile.Instance); ok && mob.Summoner != nil {
		if room != nil {
			room.Notify(event.Died{Target: mob.Name()})
		}
		w.crumble(mob)
		return
	}

	w.becomeCorpse(dead)
	if room != nil {
		// ... the Died notify, unchanged ...
	}
	// His guard goes with him, after the room has seen him fall.
	if mob, ok := dead.(*mobile.Instance); ok {
		w.crumbleSummonsOf(mob)
	}

	// ... the player branch, unchanged ...
}
```

Walkthrough points: why the summon branch returns before `becomeCorpse` (that's
where the corpse, loot and coins come from); why `crumble` ends fights with
`EndAllFightsWith` and not `EndFight` (both directions, plus retargeting -- a player
fighting the skeleton is freed, a mob it was holding turns elsewhere); why the sweep
lives in `DoViolence` (it's the only every-second pulse that's about fights, and the
fight end it catches happens there or in a handler just before it); why `roomOf` is
the removed-fighter test (`Occupancy` is the one answer to "is it in the world").

- [ ] **Step 4: run** `go test ./world ./telnet` -- PASS; `make check`.
- [ ] **Step 5: commit** -- `world: summons crumble with the fight, and a killed one leaves nothing`

---

### Task 4: `me:summon` in Lua

**Files:**
- Modify: `script/runtime.go`, `world/world.go`
- Test: `script/runtime_test.go` (harness and new tests), `world/scripts_test.go`
- Create: `testcontent/world/wrathrock/scripts/warlock.lua`
- Modify: `testcontent/world/wrathrock/mobs.json` (a `warlock`), `testcontent/world/wrathrock/instructions.json`

**Interfaces:**
- Consumes: `mobile.Definition.Summons` (Task 1); `World.summon`, `World.liveSummons` (Task 2).
- Produces: `script.Actions{Say, Summon, Summons}`;
  `script.NewRuntime(programs map[string]*Program, roller Roller, actions Actions) (*Runtime, error)`;
  `script.MaxSummonsPerCall`, `script.MaxLiveSummons`. `script.Say` is removed.

- [ ] **Step 1 (Claude): the failing tests.**

`script/runtime_test.go` harness -- `newHarness` builds `Actions`, recording summons
and answering the live count from a map the test sets:

```go
type summonCall struct {
	mob, def string
	count    int
}

type harness struct {
	t        *testing.T
	rt       *Runtime
	dice     *testdice.LoadedDice
	said     []line
	summoned []summonCall
	live     map[*mobile.Instance]int // what me.summons and the live cap read
}

func newHarness(t *testing.T, scripts map[string]string) *harness {
	t.Helper()
	h := &harness{t: t, dice: testdice.New(), live: map[*mobile.Instance]int{}}
	programs := map[string]*Program{}
	for name, src := range scripts {
		p, err := Compile(name, src)
		require.NoError(t, err)
		programs[name] = p
	}
	rt, err := NewRuntime(programs, h.dice, Actions{
		Say: func(mob *mobile.Instance, text string) {
			h.said = append(h.said, line{mob.Name(), text})
		},
		Summon: func(mob *mobile.Instance, def *mobile.Definition, count int) int {
			h.summoned = append(h.summoned, summonCall{mob.Name(), def.Id, count})
			return count
		},
		Summons: func(mob *mobile.Instance) int { return h.live[mob] },
	})
	require.NoError(t, err)
	h.rt = rt
	return h
}

// summoner is a mob running script that may summon an imp
func summoner(script string) *mobile.Instance {
	m := mob("Warlock", script)
	m.Definition.Summons = []*mobile.Definition{
		mobile.NewDefinition("imp", "imp", "wrathrock", nil, "an imp", "An imp is here.",
			5, rules.WanderDefinition{}, 10, false),
	}
	return m
}

// failures collects what a harness logs
func (h *harness) failures() *[]string {
	var logged []string
	h.rt.logFailure = func(err error) { logged = append(logged, err.Error()) }
	return &logged
}
```

New tests:

```go
func TestSummon(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say(tostring(me:summon("imp", 2))) end`})

	h.rt.FightStart(summoner("z/w"), bob)

	assert.Equal(t, []summonCall{{"Warlock", "imp", 2}}, h.summoned)
	assert.Equal(t, []string{"2"}, h.texts(), "it answers how many came")
}

// named the way mobs.json may name it: "zone/id"
func TestSummonByZoneAndId(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:summon("wrathrock/imp", 1) end`})
	h.rt.FightStart(summoner("z/w"), bob)
	assert.Equal(t, []summonCall{{"Warlock", "imp", 1}}, h.summoned)
}

func TestSummonUndeclaredFails(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:summon("dragon", 1) end`})
	logged := h.failures()

	h.rt.FightStart(summoner("z/w"), bob)

	assert.Empty(t, h.summoned)
	require.Len(t, *logged, 1)
	assert.Contains(t, (*logged)[0], "may not summon")
}

func TestSummonCountUnderOneFails(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:summon("imp", 0) end`})
	logged := h.failures()

	h.rt.FightStart(summoner("z/w"), bob)

	assert.Empty(t, h.summoned)
	require.Len(t, *logged, 1)
}

// Review focus 4: a loop gets MaxSummonsPerCall across the whole call, and
// no error -- the cap is a limit, not a mistake
func TestSummonPerCallCap(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe)
			for i = 1, 10 do me:summon("imp", 3) end
		end`})
	logged := h.failures()

	h.rt.FightStart(summoner("z/w"), bob)

	assert.Equal(t, []summonCall{{"Warlock", "imp", 3}, {"Warlock", "imp", 1}}, h.summoned)
	assert.Empty(t, *logged)
}

// and MaxLiveSummons alive at once: three standing leaves room for one
func TestSummonLiveCap(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say(tostring(me:summon("imp", 2))) end`})
	m := summoner("z/w")
	h.live[m] = 3

	h.rt.FightStart(m, bob)

	assert.Equal(t, []summonCall{{"Warlock", "imp", 1}}, h.summoned)
	assert.Equal(t, []string{"1"}, h.texts())
}

func TestSummonsReadsTheLiveCount(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me:say(tostring(me.summons)) end`})
	m := summoner("z/w")
	h.live[m] = 2

	h.rt.FightStart(m, bob)

	assert.Equal(t, []string{"2"}, h.texts())
}

func TestStashedMeCantSummonLater(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me.memory.old = me end
		function on_fight_pulse(me, foe) me.memory.old:summon("imp", 1) end`})
	m := summoner("z/w")

	h.rt.FightStart(m, bob)
	h.rt.FightPulse(m, bob)

	assert.Empty(t, h.summoned)
}

func TestSummonWithADotFails(t *testing.T) {
	h := newHarness(t, map[string]string{"z/w": `
		function on_fight_start(me, foe) me.summon("imp", 1) end`})
	logged := h.failures()

	h.rt.FightStart(summoner("z/w"), bob)

	require.Len(t, *logged, 1)
	assert.Contains(t, (*logged)[0], "colon")
}
```

Test content: `testcontent/world/wrathrock/scripts/warlock.lua`:

```lua
-- The test world's summoner: two imps the moment a fight starts.
function on_fight_start(me, foe)
  me:summon("imp", 2)
end
```

`testcontent/world/wrathrock/mobs.json`, a warlock:

```json
{"id":"warlock","name":"Warlock","short_description":"A warlock.","description_in_room":"A warlock mutters over a smoking bowl.","aliases":["warlock"],"max_health":25,"wandering_definition":{"can_wander":false},"ac":10,"script":"warlock","summons":["imp"]}
```

`testcontent/world/wrathrock/instructions.json`:

```json
{"type":"CreateMobile","mobile_id":"warlock","room_id":"smithy","instance_max":1}
```

`world/scripts_test.go`, through the real dispatch:

```go
// The warlock (testcontent) summons two imps when a fight starts: through
// kill, the runtime, the world's summon and on to their fights.
func (s *scriptsSuite) TestScriptSummons() {
	smithy := s.w.Zone("wrathrock").Rooms["smithy"]
	s.w.movePlayer(s.p, rules.DirectionNone, smithy)
	s.dice.Load([]int{0, 0}) // each imp picks the one player

	s.Require().NoError(s.w.HandleIncomingMessage(s.handlerParameter(command.Kill{Target: "warlock"})))

	imps := 0
	for _, m := range smithy.Mobiles() {
		if m.Definition.Id == "imp" {
			imps++
			s.Assert().Same(s.p, s.w.fightLedger.GetFight(m).Fightee)
		}
	}
	s.Assert().Equal(2, imps)
}
```

- [ ] **Step 2: run, see them fail** -- `go test ./script ./world`; build failures
  (`Actions` undefined; and `"summons"` in testcontent only parses once Task 1 is in).

- [ ] **Step 3 (user, walked through): the Go.**

`script/runtime.go` -- the constants, beside `MaxSaysPerCall`:

```go
// MaxSummonsPerCall and MaxLiveSummons bound what a script can bring into the
// world: per hook call, across every me:summon in it, and alive at once per
// summoner. Over either, fewer arrive -- a limit, not an error -- so a boss
// that summons every round simply stops getting more.
const (
	MaxSummonsPerCall = 4
	MaxLiveSummons    = 4
)
```

Replace `type Say` with:

```go
// Actions is the engine as a script reaches it -- what me's methods call. The
// world fills it in; a test fills it with recorders.
type Actions struct {
	// Say is me:say: the mob's room hears it.
	Say func(mob *mobile.Instance, text string)
	// Summon is me:summon, already checked against the mob's Summons and cut
	// to the caps: put count of def in the world and answer how many came.
	Summon func(summoner *mobile.Instance, def *mobile.Definition, count int) int
	// Summons is me.summons: how many of the mob's summons are alive.
	Summons func(summoner *mobile.Instance) int
}
```

`Runtime` holds `actions Actions` instead of `say Say`; `NewRuntime(programs, roller,
actions Actions)` stores it; in `fire`, the call's `say` becomes
`func(text string) { r.actions.Say(mob, text) }`. `hookCall` gains a counter:

```go
	// summoned counts this call's summons against MaxSummonsPerCall.
	summoned int
```

In `Runtime.me`, after `say`:

```go
	me.RawSetString("summons", lua.LNumber(r.actions.Summons(mob)))
	me.RawSetString("summon", L.NewFunction(func(L *lua.LState) int {
		if _, isMe := L.Get(1).(*lua.LTable); !isMe {
			L.RaiseError("summon: call it as me:summon(id, count), with a colon")
		}
		id := L.CheckString(2)
		count := L.CheckInt(3)
		if r.sb.current != call {
			L.RaiseError("summon: this me belongs to an earlier call")
		}
		def := summonable(mob, id)
		if def == nil {
			L.RaiseError("summon: %s may not summon %q -- see \"summons\" in mobs.json", mob.Definition.Id, id)
		}
		if count < 1 {
			L.RaiseError("summon: a count of %d", count)
		}
		count = min(count, MaxSummonsPerCall-call.summoned, MaxLiveSummons-r.actions.Summons(mob))
		got := 0
		if count > 0 {
			got = r.actions.Summon(mob, def, count)
			call.summoned += got
		}
		L.Push(lua.LNumber(got))
		return 1
	}))
```

And:

```go
// summonable is the definition id names among those mob may summon: its bare
// id, or "zone/id" -- either way mobs.json may have named it.
func summonable(mob *mobile.Instance, id string) *mobile.Definition {
	for _, d := range mob.Definition.Summons {
		if id == d.Id || id == d.ZoneId+"/"+d.Id {
			return d
		}
	}
	return nil
}
```

`world/world.go`, in `New`:

```go
	if w.scripts, err = script.NewRuntime(c.Scripts, liveRoller{w}, script.Actions{
		Say:     w.mobSays,
		Summon:  w.summon,
		Summons: w.liveSummons,
	}); err != nil {
```

Walkthrough points: why a struct rather than two more positional func parameters
(three funcs of similar shape in a row is how arguments get swapped); why `summon`
reads `r.sb.current != call` like `say` (a `me` kept in memory must not act later --
outside the deadline and the caps); why the caps clamp rather than raise (a boss
re-summoning every round is ordinary script, not a bug, and three failures switch the
script off); why `me.summons` is a copy (like `health`: read when `me` is built, so it
doesn't count what this call just summoned -- the caps do).

- [ ] **Step 4: run** `go test ./script ./world ./loader` -- PASS; `make check` (the
  smoke bot test loads the real content too).
- [ ] **Step 5: commit** -- `script: me:summon and me.summons, capped per call and per summoner`

---

### Task 5: The King calls his guard (Claude: Lua, test, docs)

**Files:**
- Modify: `content/world/barrow/scripts/barrow_king.lua`
- Create: `world/king_test.go`
- Modify: `CLAUDE.md` ("Scripts (Lua)"), `ROADMAP.md` ("No scripting language"), `LEVELS.md` (the King)

**Interfaces:**
- Consumes: everything above; the real content.

- [ ] **Step 1: the failing test** -- `world/king_test.go` builds a world from the
  real `content/`, as the smoke bot does:

```go
package world

import (
	"os"
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"
	"github.com/watchmud/watchmud/event"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/memstore"
	"github.com/watchmud/watchmud/mobile"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/script"
	"github.com/watchmud/watchmud/spaces"
	"github.com/watchmud/watchmud/testdice"
)

// The real Barrow-King in his real throne room, with a player in front of him.
type kingSuite struct {
	suite.Suite
	w     *World
	dice  *testdice.LoadedDice
	room  *spaces.Room
	king  *mobile.Instance
	p     *player.Player
	heard *player.Recorder
}

func TestKingSuite(t *testing.T) {
	suite.Run(t, new(kingSuite))
}

func (s *kingSuite) SetupTest() {
	content, err := loader.LoadContent(os.DirFS("../content"))
	s.Require().NoError(err)
	s.dice = testdice.New()
	s.w, err = New(content, memstore.New(), s.dice)
	s.Require().NoError(err)
	s.room = s.w.Zone("barrow").Rooms["throne_room"]
	var found bool
	s.king, found = s.room.FindMobile("king")
	s.Require().True(found)
	s.heard = &player.Recorder{}
	s.p = player.NewTestPlayer(uuid.New(), "testdood", s.heard)
	s.w.PlacePlayer(s.p, s.room)
}

func (s *kingSuite) foe() script.Foe { return script.Foe{Name: "testdood", IsPlayer: true} }

func (s *kingSuite) skeletons() int {
	n := 0
	for _, m := range s.room.Mobiles() {
		if m.Definition.Id == "barrow_skeleton" && m.Summoner == s.king {
			n++
		}
	}
	return n
}

func (s *kingSuite) said() []string {
	var lines []string
	for _, m := range s.heard.Sent {
		if e, ok := m.(event.Said); ok {
			lines = append(lines, e.Value)
		}
	}
	return lines
}

// above half, nothing: the taunt roll fails (99 is not under 15)
func (s *kingSuite) TestAboveHalfNoGuard() {
	s.king.CurHealth = s.king.Definition.MaxHealth/2 + 1
	s.dice.Load([]int{99})

	s.w.scripts.FightPulse(s.king, s.foe())

	s.Assert().Equal(0, s.skeletons())
}

// at half: his line, two skeletons, no taunt that round; and never twice
func (s *kingSuite) TestAtHalfOnce() {
	s.king.CurHealth = s.king.Definition.MaxHealth / 2
	s.dice.Load([]int{0, 0}) // each skeleton picks the one player

	s.w.scripts.FightPulse(s.king, s.foe())

	s.Assert().Equal(2, s.skeletons())
	s.Assert().Equal([]string{"Rise, my guard! Rise and defend your king!"}, s.said())

	s.dice.Load([]int{99}) // the next round's taunt roll, and no more summons
	s.w.scripts.FightPulse(s.king, s.foe())
	s.Assert().Equal(2, s.skeletons())
}

// Review focus 3: a fresh fight -- the old guard gone to dust -- gets a fresh summon
func (s *kingSuite) TestFreshFightSummonsAgain() {
	s.king.CurHealth = s.king.Definition.MaxHealth / 2
	s.dice.Load([]int{0, 0})
	s.w.scripts.FightPulse(s.king, s.foe())
	s.w.crumbleSummonsOf(s.king)

	s.dice.Load([]int{0, 0, 0}) // the opener's pick, then two targets
	s.w.scripts.FightStart(s.king, s.foe())
	s.w.scripts.FightPulse(s.king, s.foe())

	s.Assert().Equal(2, s.skeletons())
}
```

Run `go test ./world -run TestKingSuite` -- FAIL: no skeletons (the script doesn't
summon yet).

- [ ] **Step 2: the Lua** -- `content/world/barrow/scripts/barrow_king.lua`:

```lua
-- The Barrow-King. Placeholder lines: rewrite them in his voice.
local openers = {
  "Who wakes the king beneath the oak?",
  "Kneel, %s. The barrow keeps what it takes.",
}
local taunts = {
  "Hah! You cannot defeat me!",
  "I have outlasted better than you.",
  "Your bones will guard my door.",
}

function on_fight_start(me, foe)
  -- a fresh fight gets a fresh guard: he regenerates to full between them
  me.memory.risen = nil
  me:say(string.format(pick(openers), foe.name))
end

function on_fight_pulse(me, foe)
  -- at half health, once a fight, his guard rises. No taunt that round:
  -- the line and the rising are the moment.
  if not me.memory.risen and me.health <= me.max_health / 2 then
    me.memory.risen = true
    me:say("Rise, my guard! Rise and defend your king!")
    me:summon("barrow_skeleton", 2)
    return
  end
  if chance(15) then
    local line = pick(taunts)
    if line ~= me.memory.last then
      me:say(line)
      me.memory.last = line
    end
  end
end
```

- [ ] **Step 3: run** `go test ./world -run TestKingSuite` -- PASS; `make check`.

- [ ] **Step 4: the docs.**
  - CLAUDE.md, "Scripts (Lua)": actions are now `me:say` and `me:summon`; `"summons"`
    in mobs.json, resolved at load; `me.summons`; the two caps; a summon's life
    (`Summoner`, no corpse/loot/coins, not counted for resets, crumbles on the
    summoner's death or by the `DoViolence` sweep); the removed-fighter guard;
    `script.Actions` replacing `Say`. "Adding a hook" stays; add "Adding an action":
    a field on `Actions`, a method on `me` bound to the call, and the world func.
  - ROADMAP.md, "No scripting language": the King's half-health script is done
    (2026-10-04); `wait()` over coroutines is still open, with the delay between his
    line and the rising as its first use.
  - LEVELS.md, the King: his guard at half health, two skeletons, a tuning
    placeholder for when a group tries him.

- [ ] **Step 5: commit** -- `content: the Barrow-King calls up his guard at half health` (Lua,
  test, docs together).

---

## Self-review

- Spec coverage: content declaration (T1), loader failures (T1), arrive/target/nohassle
  (T2), resets (T2), crumble on death/sweep/killed (T3), API, caps, errors, stale `me`
  (T4), the King and re-summon (T5), docs (T5). `me.summons` (T4).
- Spec deviations: `Summoner` pointer for `SummonedBy`; the removed-fighter guard.
  Both above, under refinements.
- Types: `summon(summoner, def, count) int` and `liveSummons(summoner) int` match
  `Actions.Summon`/`Actions.Summons` exactly, so `world.New` passes the methods as is.
