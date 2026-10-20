# The Ruins of Emberfall Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A 30-40 room power 4-5 zone east of the Bandit Camp -- streets, sewers, the
guild compound around a sealed tower -- with portal monsters, daemonic components,
guild artifacts, and two new abilities: bolt and fireball.

**Architecture:** Tasks 1-2 are content only (`content/world/emberfall/`), held to the
world by the existing content tests plus two new ones. Task 3 adds `bolt`, smite's
effect under its own name and event. Task 4 adds `fireball` and the first target kind
that resolves to many mobs (`room`). Each task ships on master on its own.

**Tech Stack:** Go, JSON content, testify.

**Spec:** `docs/superpowers/specs/2026-10-20-ruined-town-design.md`

**Who writes what:** the owner writes the Go production code (Tasks 3-4, and any
engine change Tasks 1-2 turn out to need); Claude writes content, tests and docs,
walks through each Go change before the owner writes it, and reviews it after.

## Global Constraints

- Zone id `emberfall`, name "The Ruins of Emberfall", manifest `"power": { "min": 4, "max": 5 }`, reset `always`.
- Joined by **east from The Bandit Camp**; nothing else in the Hollowfields changes.
- Every room name unique in the world; the zone fits its grid; sewers are a level down.
- The tower: a locked door whose key is defined and placed nowhere.
- Mobs power 4-5 only. Nothing aggressive on the approach.
- No `noSell` / `noDonate` on anything in the zone.
- Components are category `OTHER` (no economy.json change: the default 10/power is already half of gear's 20).
- `keepOut` gains `{"The Bandit Camp", "east"}`.
- Bolt: id `bolt`, `foe`, mana 20, cooldown 15s, amount base 6 per_power 3 (my numbers; tune).
- Fireball: id `fireball`, target `room`, mana 40, cooldown 60s, amount base 4 per_power 2 (spec).
- Fireball never touches a player; skips `PlayerCantFight` mobs; refused in a `nofight` room; one point of wear per cast.

## Review Focus

1. **A fireball that kills a summoner along with its summons** -- the summoner's death crumbles them mid-loop; a crumbled summon must not die twice or be drawn into a fight. Test: `TestFireballKillsSummonerAndSummons` (Task 4).
2. **A fireball into a room where some mobs already fight a groupmate** -- they keep their target; only idle ones turn on the caster. Test: `TestFireballLeavesAnotherFightAlone` (Task 4).
3. **The goals ladder** now has two zones starting at 4 -- `goals` at power 1-3 must list both as next. Test: `TestGoals_theRealLadder` updated (Task 1).
4. **A wanderer or explorer at the Bandit Camp** -- must not step east once aggressive mobs exist. Test: `TestKeepOut_safe` (Task 1 adds the door, Task 2's mobs prove it).
5. **Bolt's words leaking into smite** -- a mace's smite must still say "smite". Test: render cases for both (Task 3).

---

### Task 1: The zone and the join (Claude: content and tests)

**Files:**
- Create: `content/world/emberfall/rooms.json`, `content/world/emberfall/objects.json` (the tower key only)
- Modify: `content/world/zone_manifest.json`, `content/world/hollowfield/rooms.json` (Bandit Camp: east exit; mention the road east in its description), `bot/wanderer.go` (`keepOut`, and its comment), `world/h_goals_test.go`
- Create: `loader/emberfall_test.go`

**Interfaces:**
- Produces: room ids later tasks place mobs and loot in. Naming: `road_*` approach, `street_*`/named buildings above, `sewer_*` below, `compound_*`/named guild buildings, `tower_door` the room before the tower, `tower` the sealed room. Key object id `tower_key`.

- [ ] **Step 1: Write the failing test** `loader/emberfall_test.go`:

```go
func TestEmberfall_joinsAtTheCamp(t *testing.T)   // hollowfield/bandit_camp east -> emberfall room; that room west -> bandit_camp
func TestEmberfall_theTowerIsSealed(t *testing.T) // the exit into "tower" has a door, closed and locked, key "emberfall/tower_key";
                                                  // no zone's instructions.json creates tower_key, and no mob's loot names it
func TestEmberfall_size(t *testing.T)             // 30 <= len(rooms) <= 40; at least one room is reached by "down" (the sewers)
```

- [ ] **Step 2: Run** `go test ./loader -run TestEmberfall` -- Expected: FAIL (zone not found).
- [ ] **Step 3: Write the content.** Rooms per the spec's map; the manifest entry; the camp's east exit; the tower door; `tower_key` in objects.json; `keepOut` += `{"The Bandit Camp", "east"}`.
- [ ] **Step 4: Run** `go test ./loader ./bot ./world` -- Expected: `TestEmberfall_*`, `TestGrid_theContentFits`, `TestRoomNames_uniqueInTheWorld`, `TestExplorer_mapsTheWorld`, `TestKeepOut_safe` PASS; `TestGoals_theRealLadder` FAIL (new rung).
- [ ] **Step 5: Update `TestGoals_theRealLadder`**: add `emberfall := event.GoalZone{Name: "The Ruins of Emberfall", Min: 4, Max: 5, Route: <as goals prints it, starting "5s e 2s e">}`; at powers 1-3 `Next` holds the mill and emberfall in the order `goals` gives; at 4-5 `Fits` includes emberfall. Re-run: PASS.
- [ ] **Step 6:** `make check` green; walk it with `make run` and telnet from Temple Square to the tower door.
- [ ] **Step 7: Commit** "Emberfall: the ruins east of the Bandit Camp, empty, the tower sealed".

### Task 2: Monsters and loot (Claude: content and tests)

**Files:**
- Create: `content/world/emberfall/mobs.json`, `instructions.json`; extend `objects.json`
- Modify: `loader/emberfall_test.go`

**Interfaces:**
- Consumes: Task 1's room ids.
- Produces: artifact objects later tasks add `"abilities": ["bolt"]` / `["fireball"]` to: `guild_wand` (wield, gets bolt in Task 3), `fireball_staff` (wield, defined in Task 4).

- [ ] **Step 1: Write the failing tests** in `loader/emberfall_test.go`:

```go
func TestEmberfall_mobsInBand(t *testing.T)     // every emberfall mob: 4 <= Power <= 5; none aggressive in a road_* room
func TestEmberfall_lootIsTradeable(t *testing.T) // no emberfall object is noSell or noDonate; every component is ObjectCategoryOther and unwearable
func TestEmberfall_artifactsGrant(t *testing.T) // at least 4 emberfall objects grant an ability; every ability granted is an existing one
```

- [ ] **Step 2: Run** `go test ./loader -run TestEmberfall` -- Expected: FAIL.
- [ ] **Step 3: Write the content.** Orcs (4) in the streets, lesser daemons/imps (4-5) in the sewers, wyverns (5) in the compound; at most one fleeing orc script, reusing the bandit's shape; components (orc tusk, daemon horn, ichor, wyvern scale) as loot; artifacts granting heal, ward, stun, assess, provoke, smite on a few mobs' loot at low chances, mostly in the compound; instructions.json spawning them.
- [ ] **Step 4: Tune** with `go run ./cmd/watchmud-sim`: a 4 wins against a 4 most of the time, a 5 sometimes. Put the numbers in the commit message.
- [ ] **Step 5: Run** `make check` -- Expected: green, with `TestKeepOut_safe` and `TestGrounds_safe` passing (the camp's east is shut to wanderers; bandit-track bots never leave their patrol).
- [ ] **Step 6: Commit** "Emberfall: orcs, daemons, wyverns, and what they drop".

### Task 3: Bolt (owner: Go; Claude: tests, content)

**Files:**
- Modify: `world/abilities.go`, `event/events.go`, `telnet/render.go`, `content/rules/abilities.json`, `testcontent/rules/abilities.json`, `content/world/emberfall/objects.json` (`guild_wand` grants bolt)
- Create: `world/h_cast_bolt_test.go`
- Modify: `telnet/render_test.go`

**Interfaces:**
- Produces: `event.Bolted{Actor, Target string; Damage int}`; `func (w *World) landBlow(c cast, notice func(damage int) any)` -- smite's body (damage, notify, wear, death or `openFight`), which `smiteEffect` and `boltEffect` both call.

- [ ] **Step 1: Write the failing tests.** `h_cast_bolt_test.go`, a suite shaped like `smiteSuite` with a wand granting `bolt`:

```go
TestBolt               // power 1: Bolted{Actor:"testdood", Target:"Target Drone", Damage: 9}; drone at 25-9
TestBoltStartsAFight   // caster and drone fighting each other
TestKillingBolt        // drone at 5 health dies: Bolted then Died
TestBoltWearsTheWand   // wand durability 10 -> 9
TestBoltNoFightRoom    // NoFightRoom, nothing spent
```

and in `render_test.go`: `cast bolt target` reads "You hurl a bolt of fire at Target Drone! (9)"; a bystander reads "testdood hurls a bolt of fire at Target Drone!"; the existing smite cases are unchanged.
- [ ] **Step 2: Run** `go test ./world -run TestBoltSuite; go test ./telnet` -- Expected: FAIL (unknown ability `bolt` -- testcontent first, then compile).
- [ ] **Step 3: Implement** (owner): `bolt` in both abilities.json; `event.Bolted`; `landBlow`, with `smiteEffect` rewritten on it; `boltEffect`; `"bolt"` in `effects`; the render case.
- [ ] **Step 4: Run** `make check` -- Expected: green; `TestSmiteSuite` unchanged and passing.
- [ ] **Step 5:** Claude adds `"abilities": ["bolt"]` to `guild_wand`. `make check`.
- [ ] **Step 6: Commit** "Abilities: bolt, the guild's fire at one foe".

### Task 4: Fireball (owner: Go; Claude: tests, content)

**Files:**
- Modify: `rules/ability.go` (+ `rules/ability_test.go`), `world/abilities.go`, `world/h_cast.go`, `event/events.go`, `event/result.go`, `telnet/render.go`, `telnet/resultcode.go`, both abilities.json, `content/world/emberfall/objects.json` + `mobs.json` (the staff and who drops it)
- Create: `world/h_cast_fireball_test.go`

**Interfaces:**
- Consumes: Task 3's pattern (effect, event, render); `World.openFight(c cast)`, `wearFromBlow`, `combatantDied`, `roomOf`.
- Produces: `rules.TargetRoom AbilityTarget = "room"`; `cast.foes []*mobile.Instance`; `func (w *World) castRoom(c *cast) event.ResultCode`; `event.NoFoes ResultCode = "NO_FOES"`, worded "There's nothing here to hit."; `event.Fireballed{Actor string; Targets []string; Damage int}`.

The effect's order, which the tests pin (spec: the hits, then wear, then the deaths):

```
for each foe: dead[foe] = foe.TakeMeleeDamage(damage)
room.Notify(Fireballed{...all targets...})
wearFromBlow(caster, foes[0], room)               // once per cast
for each dead foe still in a room (roomOf != nil): combatantDied
for each living foe still in a room: c.foe = foe; openFight(c)
```

- [ ] **Step 1: Write the failing tests.** `rules/ability_test.go`: `"room"` parses to `TargetRoom`. `h_cast_fireball_test.go` (start room: Target Drone and Little Drone, 25 each; fireball at power 1 = 6):

```go
TestFireballHitsEveryMob              // Fireballed{Targets: [both], Damage: 6}; both at 19
TestFireballDrawsThemAllIn            // caster fights the first; both drones fight the caster
TestFireballLeavesAnotherFightAlone   // a second player fighting Little Drone: it stays on them, Target Drone turns on caster
TestFireballNeverTouchesPlayers       // a second player in the room keeps full health
TestFireballSkipsCantFightMobs        // a PlayerCantFight mob in the room isn't in Targets
TestFireballNoFoes                    // only a cant-fight mob: NoFoes, nothing spent
TestFireballNoFightRoom               // NoFightRoom, nothing spent
TestFireballKills                     // drones at 5: Fireballed, then Died x2, in that order, no fight left
TestFireballKillsSummonerAndSummons   // w.summon(target, little's definition, 1); all at 5 health: no error logged, each Died at most once
TestFireballWearsOnce                 // staff durability 10 -> 9 with two targets
```

`render_test.go`: "You hurl a fireball! It engulfs Target Drone and Little Drone. (6)"; bystander "testdood hurls a fireball! It engulfs Target Drone and Little Drone."; `TestFailureText_everyCodeHasWords` covers `NO_FOES`.
- [ ] **Step 2: Run** `go test ./rules ./world -run 'Fireball|Ability'` -- Expected: FAIL.
- [ ] **Step 3: Implement** (owner), in the order: `TargetRoom` and its enum entry; `NoFoes` and its words; `Fireballed` and its render case; `cast.foes`; `castRoom` (nofight room → `NoFightRoom`; room's mobs minus `PlayerCantFight`; none → `NoFoes`; the typed target is ignored) and its `case` in `castTarget`; `fireballEffect` as above; `fireball` in both abilities.json and `effects`.
- [ ] **Step 4: Run** `make check` -- Expected: green.
- [ ] **Step 5:** Claude defines `fireball_staff` (wield, `"abilities": ["fireball"]`) and puts it in one compound mob's loot at a low chance. `make check`.
- [ ] **Step 6: Commit** "Abilities: fireball, which hits everything and brings it all".

### Task 5: Docs (Claude)

- [ ] CLAUDE.md: the zone in the content/world picture; bolt and fireball in "Abilities" (the `room` target kind, that every foe hit is drawn in, the summon rule); `keepOut`'s doors.
- [ ] ROADMAP.md, `docs/working.md` ("On master, not yet released"; Next: bot grounds in Emberfall, then crafting).
- [ ] `content/world/settings.json` welcome, `bot/faq.go`, `site/index.html`, `help`: wherever they name where to go after the Hollowfields. `TestFAQ_commandsTheGameKnows` green.
- [ ] `make check`; commit "Docs: the Ruins of Emberfall, bolt and fireball".
