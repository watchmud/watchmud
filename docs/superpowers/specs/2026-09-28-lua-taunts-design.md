# Lua scripting, first cut: mob taunts

Status: design, approved in conversation 2026-09-28. Not yet implemented.

## Goal

The first Lua in watchmud, scoped to the case ROADMAP.md ("No scripting language") picked
because it needs nothing new from the engine: **a mob taunts the player it's fighting.**

Success looks like: a mob that names a script says something when a fight starts and now
and then during it; every roll is testable with loaded dice; and a broken or runaway
script cannot hurt the server -- it is logged, then switched off, and the world carries on.

Scripts are written by builders and ship in the image like any other content. Players
never supply them.

## Standing rules (from ROADMAP, unchanged)

- **Go is the engine.** A script decides *when*, from actions the engine already has;
  never *how the math works*. An action a script needs that the engine lacks is a Go
  feature first.
- **One Lua state** (gopher-lua), on the world goroutine, no locks.
- **Every hook call is time-limited**, for the same reason `Send` never blocks.
- **Dice go through `w.roller`**, so scripted fights stay testable.

## Scope

In:

- two hooks, `on_fight_start` and `on_fight_pulse`
- one action, `me:say(text)`
- per-mob-instance memory, `me.memory`
- helpers `chance(pct)` and `pick(list)`
- loading scripts from content, validated at startup
- limits: time, stack, panics, a failure budget
- the Barrow-King's script

Out, deliberately (YAGNI -- each arrives when a script needs it):

- `wait()` and coroutines
- a "health crossed a line" hook, and every other hook ROADMAP lists (entered room,
  died, heard something, pulse)
- any action but `say`
- scripts on rooms or objects
- anything about the foe beyond its name and whether it is a player
- lines supplied from mobs.json to a shared script

The King's half-health script is the next case, and is what brings `wait()` and the
health hook.

## What a script sees

### Location and naming

A script is `content/world/<zone>/scripts/<name>.lua`. A mob names one in mobs.json:

```json
"script": "barrow_king"
```

A bare name is the mob's own zone; `"zone/name"` is any other zone -- the same rule loot
uses. A name that doesn't resolve fails startup.

### Hooks

Global functions the script may define. All optional.

| hook | when |
|---|---|
| `on_fight_start(me, foe)` | this mob has just entered a fight -- from not fighting to fighting -- whoever started it (`kill`, or its own aggro) |
| `on_fight_pulse(me, foe)` | once per combat round in which this mob swung, after the swing and after the room has been told about it; not called if either is dead |

A global function whose name starts with `on_` and isn't one of these fails startup, so
`on_fightstart` is caught rather than silently never running.

### `me`: the mob the script belongs to

- `me.name`, `me.health`, `me.max_health` -- copies taken when the hook is called.
  Assigning to them changes nothing in the game.
- `me.memory` -- a Lua table belonging to this one mob instance. Created empty the first
  time a hook runs for it; dropped when the mob is removed from the world. Two instances
  of one definition have two tables. Never saved.
- `me:say(text)` -- the one action. The room receives the existing
  `event.Said{Speaker: <mob name>, Value: text}`, which renders as
  `Barrow-King says, "...".`

### `foe`: who the mob is fighting

`foe.name`, `foe.is_player`. Read-only copies.

### Helpers

- `chance(pct)` -- true `pct` percent of the time: `w.roller.IntN(100) < pct`.
- `pick(list)` -- one element of a Lua sequence, chosen through `w.roller`. An empty
  list is an error (counted like any other).

`math.random` and `math.randomseed` are removed, so no roll can bypass the roller.

### `me` and `foe` are copies

Both are plain Lua tables built fresh for each call, not Go objects exposed to Lua. A
script holds copies, a `say` closure bound to its mob, and its own memory. It has no path
into the world.

### Example

```lua
-- content/world/barrow/scripts/barrow_king.lua
local openers = { "You dare wake me?", "Kneel, %s." }
local taunts  = { "Hah! You cannot defeat me!", "The barrow keeps what it takes." }

function on_fight_start(me, foe)
  me:say(string.format(pick(openers), foe.name))
end

function on_fight_pulse(me, foe)
  if chance(15) then
    local line = pick(taunts)
    if line ~= me.memory.last then
      me:say(line)
      me.memory.last = line
    end
  end
end
```

## How it fits the Go code

### Package `script/`

Owns gopher-lua and nothing else. `world` imports `script`; `script` never imports
`world`. It depends on `mobile` (for an instance's id, name and health) and on a small
value type for the foe.

- **`script.Compile(name, src string) (*Program, error)`** compiles one file and runs its
  top level, under the time limit, in the program's own environment table, so two scripts'
  globals don't collide. It checks hook names. It knows nothing of the world and is tested
  on strings.
- **`script.Runtime`** holds the one `LState`, the compiled programs by name, each mob's
  memory keyed by instance id, and a failure count per program. Built in `world.New` from
  `content.Scripts`, `w.roller`, and a say callback that sends `event.Said` to the mob's
  room. Methods, each a no-op for a mob whose definition has no script, so call sites
  never check:
  - `FightStart(mob *mobile.Instance, foe script.Foe)`
  - `FightPulse(mob *mobile.Instance, foe script.Foe)`
  - `Forget(mob *mobile.Instance)`

  `script.Foe` is `{Name string; IsPlayer bool}`, filled in by `world`.

One state across all programs is what the later `wait()` coroutines need, since
coroutines live in one state. Isolation between programs rests on the environment tables;
for builder-written scripts that is enough.

### Loading

- `loader.mobEntry` gains `Script string` (`json:"script"`).
- The loader resolves it (bare is own zone, `zone/name` any other) to a canonical
  `zone/name`, stored on `mobile.Definition.Script` -- a string, so `mobile` stays unaware
  of Lua.
- For every script referenced, the loader reads
  `world/<zone>/scripts/<name>.lua` and calls `script.Compile`. The results go into
  `loader.Content.Scripts map[string]*script.Program`. A missing file, a compile error, a
  top level that errors or times out, or a bad hook name fails `LoadContent` with the
  file named.
- Only referenced scripts are compiled. A `.lua` no mob names is ignored.

### Where the hooks fire

- **Fight start.** `h_kill` and `doMobAggro` stop calling `fightLedger.Fight` directly and
  call a new `w.startFight(attacker, defender combat.Combatant) error`. It notes which
  sides were already in a fight, calls the ledger, then calls `runtime.FightStart` for
  each side that is a `*mobile.Instance` and wasn't already fighting. Retargeting inside
  the ledger (`EndAllFightsWith`) is not a start: that mob was already fighting.
- **Fight pulse.** In `DoViolence`, after the swing, the `Struck` notification and
  `wearFromBlow`, and before the death check: if `fight.Fighter` is a
  `*mobile.Instance` and neither side is dead, call `runtime.FightPulse`. A player reads
  the blow, then the taunt. A killing blow gets no taunt.
- **Forget.** `World.RemoveMobile`, which a mob's death goes through, calls
  `runtime.Forget(mob)`, so memory lives exactly as long as the mob.

No change in `telnet/`: a taunt is an `event.Said` sent during the violence pulse, and
the prompt after each heartbeat already frames it.

## Limits

A builder writing a bad script must not be able to crash or stall the server.

- **Time.** Each hook call, and each program's top level at load, runs under
  `L.SetContext` with a **10ms** deadline.
- **Stack and registry** are capped when the state is created (`lua.Options`
  `CallStackSize`, `RegistrySize`/`RegistryMaxSize`), so runaway recursion is a Lua
  error, not an out-of-memory crash.
- **Panics.** Every call is wrapped in `recover`; a panic inside gopher-lua becomes an
  error like any other.
- **Libraries.** Only base, string, table and math are opened. Not `os`, `io`,
  `package`, `debug`, `coroutine` (until `wait()` wants it) or `channel`. From base,
  `dofile`, `loadfile`, `load`, `loadstring` and `require` are removed.
- **Failure budget.** An error or timeout is logged (zerolog: script, mob, hook, error)
  and the call returns; the mob carries on. After **3** failures a program is disabled
  until restart, with one warning when it happens. Its mobs behave as if they had no
  script.

## Testing

### `script/`, on strings, with a `testdice` roller

- `Compile` rejects: a syntax error; an unknown `on_` hook; a top level that loops
  forever (hits the deadline, doesn't hang the test).
- Two programs in one state don't see each other's globals.
- `say` receives the text; `chance` and `pick` roll through the loaded dice.
- `memory` persists across calls for one mob, is separate for two instances of one
  definition, and is gone after `Forget`.
- A hook that errors, loops forever or recurses without end is logged and returns; after
  3 failures the program is no longer called.
- `math.random`, `os`, `io`, `require` and `load` are unavailable.

### `loader/`

- A mob naming a script that doesn't exist fails the load.
- `"zone/name"` resolves across zones.
- A script that doesn't compile fails the load, naming the file.

### `world/`, with `NewTestWorld`

testcontent's target drone gets `"script": "taunter"` and a small
`testcontent/world/wrathrock/scripts/taunter.lua`: an opener, and a taunt on `chance(50)`.

- `kill target`: the room hears the opener.
- An aggressive scripted mob starts a fight on the mobile pulse; its opener fires.
- A round with loaded dice: `Struck`, then the taunt, in that order.
- A killing blow: no taunt.
- Retargeting after a death doesn't fire the opener again.
- A drone from a zone reset after the old one died starts with empty memory.

### `telnet/render_test.go`

`kill target` shows `Target Drone says, "..."`. No new renderer code; the case pins the
text.

## Content

`content/world/barrow/scripts/barrow_king.lua`: an opener and a few taunts in the shape
of the example above, and `"script": "barrow_king"` on him in barrow/mobs.json. The lines
are placeholders for the user to rewrite.

## Docs

- CLAUDE.md: a "Scripts (Lua)" section -- the boundary rule, the API, the limits, how to
  add a hook (a Go call site, a `Runtime` method, the name in the known-hook list, a test).
- ROADMAP.md: "No scripting language" notes taunts done and the King's half-health script
  as next; the first-week taunts item struck.

## Dependency

`github.com/yuin/gopher-lua`. Pure Go, no cgo; the Docker build is unchanged.
