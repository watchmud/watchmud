# Recall as a trinket -- Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> Implemented by Claude end to end (the user asked for this one to be done in one go).

**Goal:** `recall` works only for someone wearing a temple token (or a wizard), costs a
60s cooldown, and every character -- new, existing, bot -- has the token.

**Architecture:** Recall becomes the first `none`-target ability, gated by two ability
flags (`not_in_fight`, `wizards`); the `recall` command runs the same checks as `cast`.
A `backfill` flag on starting gear plus a `Backfilled` list on the record gives existing
characters the token once, at `World.Arrive`. The world clock runs at the server's tick
pace so cooldowns scale in fast test worlds; the bots retry a recall that isn't ready.

**Tech Stack:** Go, testify, the bot's telnet client.

**Spec:** `docs/superpowers/specs/2026-10-05-recall-trinket-design.md`

## Global Constraints

- `make check` green before every commit; one commit per task, on master.
- Recall: target `none`, mana 0, cooldown `60s`, `not_in_fight`, `wizards`.
- Token: `wrathrock/temple_token`, neck, `"durability": 0`, sold at the General Store,
  starting gear `equip` + `backfill`, power 1.
- Failure text: `recall/NOT_GRANTED` "Nothing you're wearing lets you recall -- a temple
  token does; the General Store sells them."; `recall/NOT_READY` "You can't recall again yet."
- Backfill message: "You find a temple token around your neck." / "You find a temple
  token in your pack. Wear it to recall."
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Ruling made while planning

**The world clock runs at the tick's pace.** Cooldowns read `World.now`, the wall clock,
while the bot tests run the game a hundred times fast (10ms ticks); a 60s cooldown there
would cost each test a real minute. `GameServer.SetTickInterval` sets
`World.SetPace(rules.PulseInterval / tick)`: production is 1×, unchanged. The grounds
test, whose world never ticks, makes its walker a wizard -- wizards have no cooldown.

## Review Focus

1. **An existing character whose neck is taken** (a bone charm) -- token in the pack, the
   message says to wear it, and recall refuses until they do. Task 3.
2. **A character who logs in twice** -- gets one token, not two. Task 3.
3. **A new character** -- worn token from creation, and no second one at their first login. Task 3.
4. **Recall mid-fight with the token** -- refused, nothing spent (no cooldown started). Task 2.
5. **The smoke run's final recall inside the cooldown** (production pace) -- waits and
   retries rather than failing the deploy. Task 4.

---

### Task 1: Content and rules

- `rules.Ability` gains `NotInFight bool` (`not_in_fight`), `Wizards bool` (`wizards`).
- `rules.StartingGearItem` gains `Backfill bool` (`backfill`).
- `content/world/wrathrock/objects.json`: `temple_token` (neck, OTHER, durability 0,
  abilities recall). Shop stock. Starting gear entry. `abilities.json` recall.
- testcontent: recall in abilities.json, a `temple_token` in wrathrock, starting gear
  entry with backfill (testcontent's kit: knife, helmet, rope → plus the token).
- Tests: `rules/ability_test.go` parses both flags; `loader` real-content test: the token
  grants recall and never wears out, and the kit has it worn and backfilled.

### Task 2: Recall through the ability checks

- `castTarget`: `case rules.TargetNone: return ""`.
- `handleCast` body moves into `w.cast(msg, abilityId, target)`; `handleCast` calls it with
  the typed ability; `handleRecall` calls it with `"recall"`. In `cast`: `granted ||
  (a.Wizards && p.IsWizard())`; the cooldown check skipped for a wizard on a `Wizards`
  ability (and no cooldown started); after `NOT_READY`, `a.NotInFight && InFight` →
  `IN_A_FIGHT`. A wizard casting without gear casts at power 0.
- `recallEffect`: `movePlayerMagically(caster, StartRoom)` + the description.
- `telnet/resultcode.go`: the two `recall/` entries.
- Tests (`world/h_recall_test.go`): with the token: lands in the start room, cooldown
  started, no mana spent; without: `NOT_GRANTED`; wizard without: works, no cooldown;
  on cooldown: `NOT_READY`; in a fight: `IN_A_FIGHT`, no cooldown started; `cast recall`
  works too. Render: the two texts. Existing recall render case wears a token.

### Task 3: Backfill at login

- `player.Record.Backfilled []string`; `Player.backfilled` + `Backfilled()` /
  `MarkBackfilled(ref)`; `Record()`/`FromRecord` carry it; `mongostore` document
  `backfilled,omitempty`.
- `GiveStartingGear` marks every `Backfill` item it hands out.
- `World.Arrive` → `w.backfill(p)` after the description: for each backfill item not in
  `p.Backfilled()`, make it (power from the kit), wear it if its slot is free else pack,
  mark it, send `event.Received{Item, Worn}`.
- Render: the two lines.
- Tests: world -- existing character, neck free: worn + message; neck taken: pack +
  message, recall refused; second Arrive: nothing; created character: nothing at Arrive.
  player: Backfilled round trip through Record/FromRecord; mongostore document round trip.

### Task 4: Pace, and the bots

- `World.SetPace(rate float64)`: `w.now` becomes start + elapsed×rate.
  `GameServer.SetTickInterval` calls it with `float64(rules.PulseInterval)/float64(tick)`.
- Bot `recall` (smoke's helper, used by tests and the smoke run) and
  `Adventurer.recall`: on "You can't recall again yet." wait 10s and retry, up to 7 tries.
- `bot/grounds_test.go`: the walker is made a wizard (startGame exposes the store).
- Tests: world -- SetPace(100): a 60s cooldown is ready after 0.6s of `now`; bot -- a fake
  server answering not-ready once then the room, recall succeeds (smoke_script_test style);
  the real-content smoke and adventurer tests stay green.

### Task 5: Docs

CLAUDE.md "Abilities": recall, the two flags, `none`, backfill, pace. LEVELS.md: recall done.
