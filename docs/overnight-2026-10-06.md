# Overnight, 2026-10-06

Unattended work on `claude/watchmud-roadmap-development-2gbc8l` (PR #36), in order, until
the session's credit runs out. Each task is its own commit with `make check` green,
pushed when it lands, so whatever is ticked here is on the branch. Nothing touches
production, and nothing here is released.

- [x] **1. Review PR #36's own diff** for bugs before anyone merges it (doors, chests,
  put, bags, the status tables, the mill) and fix what's real.
- [x] **2. `give <item|coins> to <player>`**: `object.Move` between two inventories,
  in the same room; worn things stay on, as with `drop` and `put`.
- [ ] **3. `examine <item>`**: what one thing is -- power, condition, armor, the
  abilities it grants, a bag's room -- for something carried, worn or on the floor.
- [ ] **4. A script hook for speech, `on_hear(me, speaker, text)`**, and the
  hedge-witch answering `say heal` with a line (words only: healing a player from a
  script would be a new Go action, raised rather than decided).
- [ ] **5. Small cleanup**: `world/settings.go`'s lone `VERBOSE_LOGGING` const.
- [ ] **6. Stretch: split `spaces.Room` into definition and instance** (ROADMAP "Known
  problems"): a spec first, then the refactor in steps, each green.

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
