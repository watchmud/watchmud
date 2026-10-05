# `wait()`: a hook that pauses

Status: design, approved in conversation 2026-10-04. Not yet implemented.

## Goal

The last piece ROADMAP.md ("No scripting language") named for Lua: **a hook can pause.**
The first use is the Barrow-King's -- his line, two seconds of nothing, then his guard
rises -- so the beat lands as a beat instead of all at once.

Success looks like: `wait(2)` in a hook runs the rest of it two pulses later; a mob does
one scripted thing at a time; a wait never outlives the mob or the fight it belongs to;
and every limit a hook has today -- the deadline, the caps, the three strikes, the
sandbox -- still holds across a wait.

## Standing rules (unchanged)

- Go is the engine; a script decides *when*. `wait` is the most literal *when* there is.
- One Lua state on the world goroutine, no locks. Every resume is deadlined.
- Dice through `w.roller`; the clock through pulses, so tests drive time by ticking.

## Not in scope

- Sub-second waits, waits measured on the wall clock, `wait` outside the two fight hooks
  (there are no others yet).
- Any new hook. A pulse-driven "every N seconds" hook would be its own design.

## The Lua side

`wait(seconds)` suspends the hook that calls it; the rest runs that many pulses later.
A pulse is `rules.PulseInterval`, one second, the finest grain the world has.

- A whole number from 1 to `script.MaxWait` (10). Anything else -- 0, negative, over
  10, not a number -- is a script error.
- Only inside a hook, like `chance` and `pick`. At a top level it is an error, which
  `Compile` therefore refuses.
- **After a wait, `me` is refreshed in place**: `me.health` and `me.summons` are re-read
  before the hook continues. `foe` keeps its name; that is all it carries.
- **A hook that waits is still one call.** `MaxSaysPerCall` and `MaxSummonsPerCall` span
  the whole hook, waits included, as does the stale-`me` check: a `me` from this call
  still works after a wait, one stashed from another call still doesn't.
- **`script.MaxWaitPerCall` (30 seconds, total)**: past it is an error and a strike.
  Without it, a hook that waits forever would silence its mob's script for good (below).
- Each resume runs under a fresh `CallTimeout`. The deadline limits computing, not
  waiting.

**One thing at a time.** While a mob has a hook waiting, its hooks don't fire -- those
rounds the mob swings but its script is quiet. At most one suspended hook per mob.

**What ends a wait early** -- the rest of the hook never runs, and none of these is a
failure:

- the mob dies or leaves the world (`Forget`, from `RemoveMobile`);
- its fight is over when the wait comes due;
- its program is switched off after `MaxFailures`.

Accepted: "in a fight" is checked when the wait comes due, not tracked through it. A
fight that ends and a new one that starts inside the same two seconds resumes the hook
into the new fight. Telling them apart would need a fight identity the ledger doesn't
have, for a case that changes nothing a player would notice.

The King's script:

```lua
me:say("Rise, my guard! Rise and defend your king!")
wait(2)
me:summon("barrow_skeleton", 2)
```

## The engine

**Every hook runs as a coroutine.** `fire` makes a thread (`L.NewThread`) and runs the
hook with `L.Resume(thread, fn, me, foe)` -- every hook, not only ones that will wait,
since a hook can't know in advance and Lua yields only inside a coroutine. A thread is a
small Lua stack, made only for scripted mobs, dropped when the hook finishes. Loading a
program's top level stays as it is.

**`wait` is a Go function that yields.** It checks its argument and the call's running
total against `MaxWaitPerCall`, records the pulses on the `hookCall`, and returns
`L.Yield()`. `Resume` comes back `ResumeYield`, and the runtime keeps the suspended
hook: thread, `hookCall`, `me`, mob, pulses left.

**`Runtime.Tick(inFight func(*mobile.Instance) bool)`**, once per heartbeat pulse. For
each waiting hook, in the order they began waiting (a slice, not a map), one pulse off;
for each that comes due:

- `inFight(mob)` false: dropped. The world supplies `inFight` (`roomOf(mob) != nil &&
  InFight(mob)`), so the runtime still knows nothing of rooms or fights.
- Otherwise `me.health` and `me.summons` are refreshed, `sb.current` is set back to its
  `hookCall`, and it is resumed under a fresh deadline. Its result is handled exactly
  as a first run's: finished, waiting again, or a failure counted toward `MaxFailures`.

Bookkeeping: waiting hooks are found per mob, so `fire` skips a mob that has one;
`Forget` drops a mob's along with its memory; reaching `MaxFailures` drops the
program's.

**The world:** `World.ResumeScripts()` calls `w.scripts.Tick` with the real `inFight`.
`GameServer.heartbeat` calls it every pulse, in its own `recoverPulse`, **before
violence**, so a hook that resumes into `me:summon` brings its skeletons in before the
round's swings.

## Testing

- **script/** (the harness gains a tick): `wait(2)` defers the rest exactly two ticks;
  says and summons after a wait go through; `me.health` read after a wait is the new
  value; while waiting, `FightStart`/`FightPulse` on that mob are no-ops, and fire again
  once it's done; `inFight` false at resume drops it silently -- no failure counted,
  nothing after the wait runs; `Forget` drops it; `wait(0)`, `wait(11)`, `wait("x")`
  fail, and a top-level `wait` fails `Compile`; past `MaxWaitPerCall` fails; the caps
  span a wait; a stashed `me` from another call still can't act after one; an error
  after a wait is a strike, and a disabled program's waits are dropped; two waiting
  mobs resume in the order they began. **Every existing sandbox test passes unchanged**
  -- the proof that the coroutine path is as tightly held as the old one.
- **world/**: `ResumeScripts` with the real `inFight`; the real King at half health --
  his line, nothing the next pulse, two skeletons on the second; the fight ending
  during the pause brings none. The warlock (no wait) still summons.
- **server/**: `heartbeat` resumes scripts every pulse and before violence.

## Docs

CLAUDE.md "Scripts (Lua)": `wait`, one-at-a-time, `MaxWait` and `MaxWaitPerCall`, what
ends a wait, the coroutine and deadline model. ROADMAP.md: `wait()` done. LEVELS.md: the
King's pause.
