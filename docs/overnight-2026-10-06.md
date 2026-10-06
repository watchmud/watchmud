# Overnight, 2026-10-06

Unattended work on `claude/watchmud-roadmap-development-2gbc8l` (PR #36), in order, until
the session's credit runs out. Each task is its own commit with `make check` green,
pushed when it lands, so whatever is ticked here is on the branch. Nothing touches
production, and nothing here is released.

- [ ] **1. Review PR #36's own diff** for bugs before anyone merges it (doors, chests,
  put, bags, the status tables, the mill) and fix what's real.
- [ ] **2. `give <item|coins> to <player>`**: `object.Move` between two inventories,
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
