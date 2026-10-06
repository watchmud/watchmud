# Rooms and zones: definition and instance

Status: **decided 2026-10-06: leave it for now.** Kept for when something needs it.
Proposal written overnight for the owner to decide on. **Not
started**: this changes how the world is built, and "Bigger features get a spec before
code" applies doubly to a refactor nobody asked for tonight.

## The problem, as it stands

ROADMAP "Known problems" names `spaces.Room` as the one place the Definition/Instance
split wasn't applied: static topology (`Id`, `Name`, `Description`, `flags`, exits)
and live contents (`playerList`, `Inventory`, `mobs`) in one struct, with `Connect`
exported so the loader can wire exits.

Looking again, it's wider than the room:

- **`spaces.Zone` conflates the same way.** `ObjectDefinitions`, `MobileDefinitions`,
  `Commands`, `ResetMode`, `Lifetime`, `Power` and `Shops` are content; `Rooms`,
  `Doors` (each a live `lock.Lock`) and `LastReset` are the running world.
- **So `loader.Content` isn't immutable.** CLAUDE.md says `Content` is static
  definitions and `World` holds instances, but `world.New` resets
  `content.Zones[...]` in place and plays the game in `content`'s rooms. Two worlds
  built from one `Content` would share every room, door and floor. Nothing does that
  today only because every test and the server call `LoadContent` once per world.
- **The writers are already loader-only.** `Connect`, `SetFlags` and `SetDoor` have no
  caller outside `loader/`. So the risk the ROADMAP names -- a handler rewriting
  topology -- is real in the type system, not in the code. That makes this a
  correctness-of-design change, not a bug fix, and it can wait for a reason.

## Proposal

Do the split the ROADMAP describes, at the zone as well as the room:

- **`spaces.RoomDefinition`**: `Id`, `Name`, `Description`, flags, and exits as
  references -- direction, `"zone/room"`, and a door spec (name, aliases, initial
  `lock.State`, key). Owned by a **`spaces.ZoneDefinition`**, which keeps everything
  `Zone` holds today that is content. Built by the loader, never written after.
- **`spaces.Room`** (live): points at its `RoomDefinition`; holds the players, mobs and
  floor, and its resolved exits (`map[Direction]*Room`) and doors (`*Door`).
- **`spaces.Zone`** (live): points at its `ZoneDefinition`; holds its `Rooms`, `Doors`
  and `LastReset`; `Reset` replays the definition's commands into its own rooms.
- **`spaces.Build(defs) (map[string]*Zone, error)`** makes the live zones from the
  definitions in two passes, as the loader does now -- every room, then every exit and
  door -- and is the only code that connects rooms. `Connect` and `SetDoor` become
  unexported. Door pairing (one door on both sides) moves from `loader/doors.go` into
  `Build`; the loader keeps the *checks* (declared twice, locked without a key, key
  not an object).
- **`world.New`** calls `Build` and owns the result. `loader.Content` is then truly
  immutable, and a test could load content once and build a fresh world from it per
  test -- the bot and telnet suites load all of `content/` dozens of times today.

Reading code barely changes: `room.Name` becomes `room.Name()` (or
`room.Definition.Name`), and the ~150 call sites are mechanical, compile-checked.

## Plan, each step green and its own commit

1. `RoomDefinition`/`ZoneDefinition` types; the loader builds them *beside* today's
   rooms. No behaviour change; a test that every live room matches its definition.
2. `spaces.Build`, with door pairing moved into it; tests in `spaces`.
3. `world.New` builds from definitions; `loader.Content` stops holding live zones.
   Shops, starting gear and the settings' room refs move to definition lookups.
4. `Room`'s static fields become reads through its definition; `Connect`/`SetDoor`/
   `SetFlags` unexported or gone.
5. Tests that load content once (`sync.OnceValue`) and build per test, where it helps.
6. Docs: CLAUDE.md "Content loading" and "Definition vs Instance", ROADMAP.

## Alternatives

- **Seal the room in place**: unexport the fields, add reads, keep one struct. Cheap,
  closes the type-system hole, leaves `Content` mutable and the zone conflated.
- **Leave it.** The writers are loader-only and nothing builds two worlds. Honest
  option; the ROADMAP entry stays.

Recommendation: the split, when something wants it -- a second world per test, a
builder command that edits rooms live, or the object index ROADMAP step 5 talks
about. Not before.
