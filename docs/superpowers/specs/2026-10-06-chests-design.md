# Chests

Status: design, 2026-10-06. The second user of `lock.Lock`, after doors.

## Goal

A container in the world -- a chest, a strongbox, a grain bin, whatever the text calls it
-- that can be closed, locked, and unlocked with a key, and that zone resets fill and
lock again. `get <item> from <chest>` and `look in <chest>` work as they do on a corpse,
once it's open.

## What a chest is

An object whose definition has a `"container"`:

```json
{ "id": "strongbox", "name": "miller's strongbox", ...,
  "container": { "closed": true, "locked": true, "key": "strongbox_key" } }
```

- Every instance gets its own `Contents` and its own `*lock.Lock` (`Instance.Lock`), so
  two strongboxes are two locks. A corpse is a container with no lock: always open.
- **It stays where it is.** A container definition is `noTake`: a chest on the floor is
  furniture. That's also what keeps lock state and contents out of the player record --
  floors aren't saved.
- The key resolves the way a door's does; the same load checks (locked needs closed and
  a key; the key must be an object).

## Commands

`open`/`close`/`lock`/`unlock` look for a door first and then for a lockable container
on the floor, by name or alias -- so `open chest` and `open west` both work, and one
handler serves both. A chest's change is `event.ContainerChanged{Actor, Container,
Change}`, seen by the room. `get from` and `look in` refuse a closed one:
`CONTAINER_CLOSED`, "It's closed."

## Resets

`CreateObject` gains two optional fields:

- `"container": "<object id>"` puts the object inside the first container of that
  definition in the room instead of on the floor.
- `"power"`: what it's made at. Otherwise the zone band's bottom -- what a mob with no
  power is, too.

And `instance_max` now means something for objects: a reset tops a room (or a
container) up to it rather than adding another every time. It never did: every reset
of the Sample Zone added another fountain. A reset that finds the chest already there
resets its lock to how the definition starts it.

## Content

The Drowned Mill: a **grain bin** in the loft, closed but not locked, holding a small
iron key; and the **miller's strongbox** in the wheel pit, locked, opened by that key,
holding a river-stone pendant and eelskin boots at the top of the band. The grate gets
you to the miller; the loft gets you into his strongbox.

## Not now

Trapped or breakable chests. (`put <item> in <chest>` came straight after: get-from
backwards, coins included. Then bags: `"portable": true`, no lid, carried and saved one
level deep -- containers don't nest -- with a `"capacity"`; see CLAUDE.md.)
