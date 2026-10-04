# The Barrow-King's guard: `me:summon`

Status: design, approved in conversation 2026-10-04. Not yet implemented.

## Goal

The second Lua case ROADMAP.md ("No scripting language") named: **at half health the
Barrow-King calls up his guard.** He says something and two barrow skeletons rise, each
going for a player in the room. It is the King fight's spike, the moment the group's
tank, healer and damage -- and provoke and stun -- earn their keep.

Success looks like: the King summons exactly once per fight, at half health; his
summons fight the group and are gone when his fight is; nothing about a summon can be
farmed, block a respawn or outlive the fight; and a buggy script still can't flood the
world with mobs.

## Standing rules (unchanged)

- **Go is the engine; a script decides *when*.** Summoning is a Go action the script
  calls. Who a summon fights, what it drops and when it goes are engine rules, not Lua.
- One Lua state on the world goroutine, every call deadlined, dice through `w.roller`.

## Not in scope

- **A "health crossed a line" hook.** `on_fight_pulse` already sees `me.health` and
  `me.max_health` every round and `me.memory` remembers that he has summoned; a check
  in Lua is enough.
- **`wait()` over coroutines.** A beat between the call and the rising would be nice
  drama; the scene works without it. Still open in ROADMAP.
- **Summons for anyone but mobs with a script**, any other script action, a boss
  immune to stun.

## Script API

`me:summon(mob_id, count)` -- bound to the one call like `me:say`, so a `me` kept in
memory can't summon later. Returns how many arrived.

`me.summons` -- how many of this mob's summons are alive right now (a copy, like
`me.health`).

**What a mob may summon is content.** mobs.json gets `"summons": ["barrow_skeleton"]`,
resolved like loot: a bare id is the mob's own zone, `"zone/id"` any other. The loader
fails startup on one that doesn't resolve. `me:summon` of an id not in the list is a
script error, and counts toward `script.MaxFailures`.

**Caps**, because a buggy script loops:

- `script.MaxSummonsPerCall` (4): the most one hook call brings in, across all its
  `me:summon` calls.
- `script.MaxLiveSummons` (4): the most one mob has alive at once.

Over either, fewer arrive and the return value says how many. Neither is an error. A
`count` under 1 is an error.

## The King's script

```lua
function on_fight_start(me, foe)
  me.memory.risen = nil
  me:say(string.format(pick(openers), foe.name))
end

function on_fight_pulse(me, foe)
  if not me.memory.risen and me.health <= me.max_health / 2 then
    me.memory.risen = true
    me:say("Rise, my guard! Rise and defend your king!")
    me:summon("barrow_skeleton", 2)
    return
  end
  -- the taunts, as now
end
```

He regenerates to full out of a fight, so every new attempt gets the summon once.
Tuning placeholder: two skeletons (35 hp, AC 14, 1d8). The real number waits for a
group to try him (LEVELS.md).

## Engine

`script.Runtime` gets a `Summon func(mob *mobile.Instance, def *mobile.Definition,
count int) int` callback beside `Say`; the runtime resolves the id against the mob's
declared `Summons` and applies the caps, and `World.summon` does the rest.

**Arriving.** Each summon is `mobile.NewInstance(def)` with `SummonedBy` set to the
summoner's id -- a field on `mobile.Instance`, in memory only like everything about a
mob -- placed in the summoner's room through `Occupancy`. The room is sent
`event.Summoned{Summoner, Name, Count}`: "The Barrow-King calls, and 2 barrow
skeletons rise from the earth!"

**Fighting.** Each summon picks a player in the room at random through `w.roller`,
skipping a wizard with `nohassle` the way aggro does, and `startFight`s them. None to
pick: it stands, and crumbles with the rest.

**Crumbling.** A summon lives exactly as long as its summoner's fight.

- *The summoner dies*: in `combatantDied`, right after his `event.Died`,
  each of his summons crumbles.
- *His fight ends any other way* (fled, wiped, slain by a wizard -- `slay` goes through
  `combatantDied`, so it's the case above): a sweep at the end of `DoViolence` crumbles
  any summon whose summoner is gone or not `InFight`. One check covers every way a
  fight can end, including ones not written yet.
- *A summon is killed*: `event.Died` as for any kill, then it crumbles instead of
  becoming a corpse -- no corpse, no loot, no coins.

To crumble: `event.Crumbled{Name}` to the room ("A barrow skeleton crumbles to dust."),
`EndAllFightsWith` (so retargeting runs as for any death), `RemoveMobile`.

**Respawns.** `Occupancy.MobileCount` skips summoned mobs, so a summon mid-fight never
keeps a zone reset from refilling the antechamber.

## Testing

- **script/**: `me:summon` reaches the callback and returns its answer; an undeclared
  id and a count under 1 are failures; the per-call and live caps cut the count; a
  `me` from an earlier call can't summon; `me.summons` reads the live count.
- **loader/**: `"summons"` resolves bare and `zone/id`; an unknown one fails the load.
- **world/**: summons arrive in the summoner's room, each fighting a player picked by
  loaded dice; a `nohassle` wizard is skipped; the summoner's death crumbles them, in
  order -- his `Died`, the crumbles; the sweep crumbles them once his fight ends
  another way; a killed summon leaves no corpse and no loot; `MobileCount` ignores
  them. In `scripts_test.go`, the real King script summons exactly once at half health,
  and again in a fresh fight.
- **telnet/**: `render_test.go` cases for `Summoned` and `Crumbled`.
- The smoke bot's in-process test loads the real content, so a broken King script
  fails `make check`.

## Docs

CLAUDE.md "Scripts (Lua)": the action, `"summons"`, the caps, the lifecycle. ROADMAP.md:
the King's half-health script done, `wait()` still open.
