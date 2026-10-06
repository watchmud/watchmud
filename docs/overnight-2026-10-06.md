# Overnight, 2026-10-06

Unattended work on `claude/watchmud-roadmap-development-2gbc8l` (PR #36), in order, until
the session's credit runs out. Each task is its own commit with `make check` green,
pushed when it lands, so whatever is ticked here is on the branch. Nothing touches
production, and nothing here is released.

- [x] **1. Review PR #36's own diff** for bugs before anyone merges it (doors, chests,
  put, bags, the status tables, the mill) and fix what's real.
- [x] **2. `give <item|coins> to <player>`**: `object.Move` between two inventories,
  in the same room; worn things stay on, as with `drop` and `put`.
- [x] **3. `examine <item>`**: what one thing is -- power, condition, armor, the
  abilities it grants, a bag's room -- for something carried, worn or on the floor.
- [x] **4. A script hook for speech, `on_hear(me, speaker, text)`**, and the
  hedge-witch answering `say heal` with a line (words only: healing a player from a
  script would be a new Go action, raised rather than decided).
- [x] **5. Small cleanup**: `world/settings.go`'s lone `VERBOSE_LOGGING` const.
- [x] **6. Stretch: split `spaces.Room` into definition and instance** (ROADMAP "Known
  problems"): a spec first, then the refactor in steps, each green. *Spec only -- see
  notes.*

- [ ] **7. FAQ answers for the new commands** (give, look/examine, bags), so the
  socialite documents them for the newest players.
- [ ] **8. Hunt the two flaky tests** seen once each earlier (a bot game test,
  `TestMoveWhileFighting`): run the suite many times, root-cause what fails.

Feel and tuning calls met along the way are noted below rather than decided.

## Notes

- **1.** Two fixes, both in zone resets. A missing `instance_max` meant "another every
  reset" for an object and "never" for a mob; every instruction in content sets it, so
  nothing was wrong yet, and the loader now refuses one under 1. And a reset counted
  objects by definition id alone, so another zone's `key` filled this zone's quota; it
  counts the definition now. Read and found sound: put, get-from, findContainer, the
  door handler and its lock errors, bag persistence (writebehind compares records with
  `reflect.DeepEqual`, so a change inside a bag is saved).
- **2.** `give` is put towards a player. To fit `help` on one screen (45 lines, a test)
  `inventory` and `equipment` now share a line. `give coins to bob` with no number gives
  every coin, as `put coins in chest` does -- consistent, but easy to fat-finger;
  requiring a number is a one-line change if that reads as a trap. The site guide
  isn't touched (it waits for a release).
- **3.** Found that `look <thing>` was in `help` but did nothing -- the handler showed
  the room whatever you named. It now looks at a player, a mob, or an object (carried,
  worn, then the floor), with `examine`/`exa` as aliases. A mob shows how hurt it looks
  in words (perfect / slightly hurt / wounded / badly hurt / near death at 100/75/50/25)
  and nothing assess gives; the bands are a feel call, easy to move.
- **4.** `on_hear(me, speaker, said)`, with `said.words` as a set because scripts
  have no `string.find`. Mobs don't hear mobs. The hedge-witch answers heal / healing /
  hurt / wounded / mend with two placeholder lines -- the second tells players she
  carries a censer they'd have to take off her, which is true (she drops it, 15%) but
  is a nudge to kill her: reword it if that's not wanted. No new Go action: she talks,
  she doesn't heal.
- **5.** `VERBOSE_LOGGING` gone; ping logs at trace. The stdlib-`log` half of the
  ROADMAP item had already gone everywhere but `cmd/watchmud-bots`.
- **6.** Spec, not code: `docs/superpowers/specs/2026-10-06-room-definition-split-design.md`.
  Looking closer, `Zone` conflates content and live state too, and `loader.Content`
  isn't immutable (the world plays in its rooms) -- but the writers are loader-only,
  so nothing is broken. ~150 call sites; a design to agree before doing, with a
  recommendation to wait until something needs it.
