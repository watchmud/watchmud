# Inhabitant bots: adventurers, openly labelled

Status: design, approved in conversation 2026-09-30. Not yet implemented.

## Goal

ROADMAP.md, "Then: content and bots", the third use: **make an empty world feel
inhabited.** Player counts will be low almost all the time, so a few bots are always on,
doing what players do -- walking out to the Hollowfields, fighting, looting, resting,
coming back to town -- at a human pace, and saying something now and then.

Success looks like: a player who logs in at 3am sees `Wren [bot]` in `who`, passes Pim
fighting a rat in the hedgerow, finds a feather Odo donated in the donation room, and at
no point mistakes any of them for a person.

## Standing rules

- **Openly bots.** Every bot is labelled wherever a player sees a character list, and a
  `tell` to one gets an honest answer. A bot never passes as a person.
- **The label is the server's, not the bot's.** It is a flag on the record, set by hand,
  like `Wizard`. A client can't declare itself a bot or decline to be one.
- **A bot is a player** (from the smoke-bot spec): plain telnet through `bot.Client`, the
  text a player reads, nothing of the server imported.
- **Manners before fun.** Bots never compete with a real player for a kill.
- **Go is the engine.** A bot is a client and can only do what a player can type.

## Scope

In:

- dropped items decay (a prerequisite -- see below)
- the `Bot` flag, `[bot]` in `who`, `help bots`
- `bot.Client.ReadChunk`
- the adventurer: the brain, hunting grounds, manners, rest, donation, talk
- `cmd/watchmud-bots` and a `bots` compose service
- Tester flagged as a bot

Out, and wanted later (ROADMAP gets these as future work):

- **Wanderer**: a bot that roams and looks around, fights only when attacked, never loots.
- **Socialite**: a bot that stays in town, greets new characters and answers a few
  newbie questions.
- **Exploring and mapping** instead of hand-written hunting grounds: learn the world
  from room names and exits, so a new zone needs no code. This replaces the tables when
  it lands. It needs room names to stay unique, which is true today but not enforced.
- **Gear**: bots wearing upgrades they find, and so growing into the Barrow.

## Prerequisite: dropped items decay

Nothing ever leaves a floor except a corpse. Three bots donating around the clock would
pile hundreds of items a day into the donation room, until `look` there is a wall of text
and memory grows until the next restart. Players dropping junk is the same problem,
slower.

- **`rules.DroppedDecay` = 30 minutes.** `handleDrop` sets `DecaysAt` on each object it
  moves to the floor, and `get` clears it, so carrying is never decay and dropping again
  restarts the clock.
- **The existing sweep already does the rest.** `World.decayCorpses` removes anything on
  a floor whose `DecaysAt` has passed and tells the room (`A long goose feather crumbles
  to dust.`). It is renamed `DecayFloors`, since it isn't only about corpses any more.
- **Zone-reset objects don't decay**: they never go through `drop`. Nor does anything a
  wizard `load`s. Only what a character drops.
- 30 minutes is long enough for a newbie to find a donation and short enough that the
  room turns over. It's a tuning number in LEVELS.md, like `CorpseDecay`.

## The Bot flag

- `Bot bool` on `player.Record`, on `mongostore`'s document (`bot`), and
  `Player.IsBot()`, mirroring `Wizard` exactly. It's set while logged out, by
  `make bot NAME=... [UNSET=1]` against the dev mongo, and by the mongosh line in
  deploy/README.md in production.
- `event.WhoEntry` gains `Bot bool`; `renderWho` appends ` [bot]` to the name. Nothing
  else changes. Room lines (`Wren is here.`) stay as they are: `who` is where players look
  for who is around.
- **`help bots`.** Help gains topics: `help <topic>` answers a paragraph from a map in
  `telnet/help.go`, and the command list ends with `help bots   who the [bot]
  characters are`. The paragraph says the bots are programs, say what they do, and
  mention that they can't chat.
- **Tester gets the flag** in production. It shows up in `who` for a few seconds every
  deploy.

## `bot.Client.ReadChunk`

```go
func (c *Client) ReadChunk(timeout time.Duration) (Chunk, error)

type Chunk struct {
	Text      string // everything up to the prompt, prompt removed
	Health    int    // from the prompt: <Health/MaxHealth hp>
	MaxHealth int
}
```

Every burst of server output ends in a prompt, so a chunk is one unit of perception: a
room, a round of combat, a tell. It consumes what it returns, like `Expect`, and the two
mix freely. On a timeout with no prompt yet, it returns an empty `Chunk` and no error,
leaving any partial text for the next call: quiet is not a failure for a bot that is
resting. A closed connection is an error.

## The adventurer

One goroutine per bot, a loop: read whatever arrived, handle interrupts, choose one
command, send it, wait a human pause.

### Interrupts, checked in every chunk first

- **`X tells you, "..."`** → `tell X I'm a bot (see 'help bots') - I can't chat, sorry!`.
  At most once per sender per 10 minutes, and never to a sibling bot (the bots know each
  other's names from their config), so two bots can't talk each other into a loop.
- **Attacked** (a combat line naming it as the target while it isn't fighting) → it
  fights back.
- **`You are dead!`** → it wakes in Temple Square at 1hp: rest, then start again from
  town.

### States

- **Town.** `equipment` for its power (`You are using (power N)`), then the first hunting
  ground whose band contains N and which it isn't avoiding. With none, it idles in town
  and tries again in a few minutes.
- **Travel.** The ground's route from Temple Square, a room at a time, checking each
  room name as it goes (as `Smoke` does).
- **Hunt.** It walks the ground's patrol rooms in a loop. In each one:
  - if a real player is there (a `X is here.` line whose name isn't a sibling), it
    **avoids this ground for 10 minutes** and goes back to Town;
  - otherwise, for each mob line matching the ground's prey table, `consider <keyword>`,
    and `kill <keyword>` if the answer is "looks like a fair fight" or easier;
  - after a kill, `get all from corpse`, counting the `You get` lines.
- **Fight.** Wait for chunks until a death line. `flee` below 25% health, then Rest.
- **Rest.** Below 50% health, outside a fight: stay put, reading chunks, until 90%. Regen
  is 5% of max every 5 seconds, so about 40 seconds.
- **Donate.** Once it has looted 5 items: `recall`, `east` to the donation room,
  `drop all.<keyword>` for each loot keyword of the ground, then back to Town.

### Hunting grounds: Go tables in `bot/`

```go
type ground struct {
	name     string
	minPower int
	maxPower int
	route    []step // from Temple Square
	patrol   []step // a loop, ending where it starts
	prey     []prey
	loot     []string // keywords to drop when donating: "feather", "pelt", ...
}
type step struct{ dir, room string }
type prey struct{ keyword, seen string } // "goose", "An angry goose lowers its neck"
```

v1 has one ground, the Hollowfields (power 1-5), and the bots stay there: they donate
everything they find, so they stay at starting power. The Barrow is a second entry once
gear exists.

### Talk

Each bot has its own phrase list per event: a kill, arriving in town, resting, donating.
It speaks on about 1 in 5 of those events, and never more than once every 10 minutes. The
lists are Go, in `bot/`, one per bot name. A name without a list is a quiet bot, not an
error.

### Pace

2-6 seconds between commands and 1-3 seconds per room walked, both jittered, with each
bot's randomness seeded differently, so no two move in step. A person at a keyboard.

## `cmd/watchmud-bots`

- `WATCHMUD_BOTS=Wren,Pim,Odo` and `WATCHMUD_BOTS_PASSWORD` (one password for all).
  Flag `-addr` (default `localhost:4000`).
- One goroutine per bot. A bot that is disconnected or errors logs why, waits 30 seconds
  to 2 minutes (jittered), and logs back in, forever. Start-up is staggered the same way,
  so a deploy doesn't bring all three in at once.
- Logs a line per state change (`Pim: hunting the Hollowfields`), never per command.
- No bots configured: log that once and idle, rather than exit and have
  `restart: unless-stopped` spin.
- At most 5 bots: they share one container address, and the server allows 5 connections
  per address. It refuses to start with more.

## Deploying

- **Dockerfile** builds `/app/watchmud-bots` beside the other two.
- **compose.yaml** gets a `bots` service: same image, entrypoint `/app/watchmud-bots
  -addr watchmud:4000`, `depends_on: watchmud`, `restart: unless-stopped`, the two
  variables from `.env`, and json-file logging like the game. `deploy.sh`'s `up -d`
  restarts it onto each new version with no other change.
- **The smoke test is unaffected.** Tester reaches the millpond within a second of the
  restart, while the bots are still logging in at human pace. Tester isn't in the
  bots' name list, so they treat it as a player and give it room.
- **deploy/README.md, "Bots"**: creating each character by hand, the mongosh line
  setting `bot: true` (Tester included), and the two `.env` keys.

## Testing

- **World:** a dropped item gets a `DecaysAt` and is swept after `DroppedDecay`; picking
  it up clears it; a zone-reset object never decays. `who` shows `[bot]` for a flagged
  player; the flag survives a save and load in memstore and mongostore.
- **telnet:** `help bots` renders; `help_test.go` still holds help to the parser.
- **bot, against the scripted fake**: `ReadChunk` splits on prompts and reads health; a
  tell gets the bot reply once per sender per 10 minutes, and a sibling's tell gets none;
  attacked → fights back; death → rest → town; a player in a patrol room → avoid and
  leave; below 25% → flee; five items looted → donate.
- **bot, in-process against the real content** at fast ticks (as `Smoke` is): every
  ground's route and patrol walks; and one adventurer, with its pauses turned down,
  runs until it has killed, looted and donated, and has answered a tell from a test
  player.

## Order of work

1. Dropped items decay (engine; useful on its own).
2. The bot flag, `who`, `help bots` (engine).
3. `ReadChunk` and the adventurer (bot/).
4. `cmd/watchmud-bots`, compose, the README.
5. By hand on production: create the bots, flag them and Tester, fill in `.env`.

## Docs

- ROADMAP.md: inhabitants done; Wanderer, Socialite, mapping and gear as future work.
- LEVELS.md: `DroppedDecay` beside the other tuning numbers.
- CLAUDE.md: the `Bot` flag beside `Wizard`; dropped-item decay beside corpses; the
  adventurer under `### bot/`.
