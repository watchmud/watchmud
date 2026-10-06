# Doors, locks and keys

Status: design, 2026-10-06. Doors first; the same lock goes on chests next.

## Goal

An exit can have a door that is open or closed, and a closed door can be locked. A
locked door opens to whoever carries its key. Content decides where doors are, what
they're called, how they start, and which object is the key; zone resets put them back.

Success: a player can be stopped by a closed door, open it, lock it behind them with the
key, and find it locked again after the zone resets -- and the same lock, unchanged,
can later sit on a chest.

## The lock is its own thing

`lock.Lock` (new leaf package) is the state and the rules, knowing nothing of rooms or
objects: `Closed`, `Locked`, the `Key` that fits ("zone/object"), and the state it
starts in. Four operations, each answering why not if it can't:

| | refused when |
|---|---|
| open | already open; locked |
| close | already closed |
| lock | already locked; not closed; no key |
| unlock | not locked; no key |

`Reset` puts it back to how it started. A lock with no key can be opened and closed but
never locked or unlocked -- a plain door. Whether the actor holds the key is the
caller's question (does anything carried have that definition?), passed in as a bool,
so the package stays a leaf a chest can use as it is.

## Doors

A door is an exit's, shared by both sides: `spaces.Door` is a name, aliases and a
`lock.Lock`, and both rooms' exits point at the same one, so opening it from the yard
opens it from the house. In rooms.json a door goes on an exit:

```json
{ "direction": "west", "dest_room": "wheel_pit",
  "door": { "name": "iron grate", "aliases": ["grate"], "closed": true,
            "locked": true, "key": "grate_key" } }
```

Declared on one side, it's found on both: the loader pairs each door with the reverse
exit, if there is one. Declaring it on both sides is an error -- two places to keep in
step. A key is an object id, bare for the door's zone or "zone/id", resolved at load
like loot: a key that doesn't exist fails startup, as does `locked` without `closed`
or without a key.

## What a closed door changes

- **Moving** through it is refused: `DOOR_CLOSED`, "The way is shut."
- **Fleeing** can't pick it; **mobs** never wander or path through it.
- **The exits line** says so: `[ Exits: North, West (closed) ]`, and `exits` too. The
  wanderer bot reads "(closed)" and doesn't try it.
- **GMCP** `Room.Info` still lists the exit: a map should show where doors are.

## Commands

`open <door>`, `close <door>`, `lock <door>`, `unlock <door>`. The target is the door's
name or an alias (`open grate`) or a direction (`open west`); `door` finds the only door
in the room. One event, `event.DoorChanged{Actor, Door, Direction, Change}`, goes to
both rooms: the actor's room sees who did it; the far side sees "The iron grate to the
east swings open." without a name. Failures are the usual `event.Failed` codes:
`NO_DOOR`, `ALREADY_OPEN`, `ALREADY_CLOSED`, `ALREADY_LOCKED`, `NOT_LOCKED`, `LOCKED`,
`NOT_CLOSED`, `NO_KEY`.

## Resets

`Zone.Reset` resets the doors of its rooms. A door between two zones belongs to the
zone that declares it.

## Content

The Drowned Mill gets the first: an iron grate between the Flooded Cellar and the Wheel
Pit, closed and locked; the key, a rusted key, drops from drowned millhands (25%). The
miller is a door away, and getting to him is a hunt first.

## Not now

Chests (next: the same `lock.Lock` on a container), picking locks, keys that break or
are used up, doors only one side can open, hidden doors.

# Plan

1. `lock/`: the type and its four operations, `Reset`, tests.
2. `spaces`: `Door`; `Room` keeps a door per direction beside the exit; `Passable`,
   `DoorTo`, `FindDoor`; `ExitString` marks closed; `Zone.Reset` resets doors.
3. `loader`: the `door` key on an exit, pairing, key resolution, the checks.
4. `world`: `handleMove`, flee and mob wandering respect closed doors; `exits` marks
   them; `open`/`close`/`lock`/`unlock` handlers through one `doorCommand`.
5. `command`, `event`, `telnet/parse.go`, `render.go`, `resultcode.go`, `help.go`.
6. `bot`: the wanderer skips closed exits.
7. Content: the grate and its key in the mill; `TestDrownedMill_walk` unlocks it.
8. Docs: CLAUDE.md, LEVELS.md, ROADMAP.md, working.md.
