# The Wanderer

Status: design, 2026-10-06. ROADMAP "Inhabitants": "a **Wanderer** (roams, fights only
when attacked, never loots)".

## Goal

A bot that makes the world look walked-in rather than farmed. The hunters stay on two
loops in the fields; a wanderer turns up anywhere it's safe to be -- the market, the
witch's hollow, the bandit track -- passes through, sometimes stops a while, and moves
on. It never starts a fight and never takes anything.

## Behaviour

- **Roams by the exits it reads.** In each room it takes a random exit from the
  room's `[ Exits: ... ]` line, never straight back the way it came unless that's the
  only way out (a dead end like the smithy). No map, no routes: what it knows is what a
  player reading the room knows.
- **Keeps out of what would kill it.** A wanderer is a power-1 character like every
  bot. `keepOut` is a hand-written list of doors (a room and a direction) it never
  takes: today one, south from the Edge of the Old Wood, which shuts off the wolves,
  the Overturned Oak and the whole Barrow behind them. `TestKeepOut_safe` walks the real
  content from the start room without those doors and fails if anything aggressive
  above power 2 can be reached -- so new content behind a door that isn't listed fails
  `make check` instead of killing wanderers in production.
- **Lingers** in one room in four, for `Pace.Linger` (30s-2m at human pace), answering
  tells and fighting back as it waits.
- **Fights only when attacked** (the goose, the rabbit), with the hunter's procedure:
  fight back, flee below 25%, rest below 50% where it stands. Never `kill`, never
  `consider`, never loots -- a corpse it leaves is anyone's.
- **Death and getting lost** are the hunter's: recall, and set off again from home.
- Its manners are the hunters': the same honest answer to tells, the same rare
  remarks (a new moment, `momentWander`).

## Shape

Not a new type: a wanderer is an `Adventurer` with `AdventurerConfig.Wander` set, whose
home state leads to `wander` instead of `town`. Everything it shares with a hunter --
reading chunks, tells, fighting back, resting, recall, dying -- is the same code.

`cmd/watchmud-bots` reads `WATCHMUD_WANDERERS` beside `WATCHMUD_BOTS`: more names, same
password, the same cap of five in all, and every one a sibling of every other, so a
hunter never makes way for a wanderer.

## Not now

Wanderers that follow roads rather than stumbling, or prefer rooms with people in them;
any memory of where it has been. Exploring and mapping is its own ROADMAP item.
