# Hunters climb the Hollowfields

Status: **design, 2026-10-18.** Option 2 of
`2026-10-10-bots-that-grow-and-coin-sinks-design.md`, after option 1 (hunters buy
the General Store's kit, `2026-10-15-bots-buy-their-kit-design.md`) shipped in
v0.12.0.

## Why

A hunter that has bought the store's kit is power 3 and still farms rats and geese.
Nothing about it changes after that. This change lets a grown hunter move up the
Hollowfields -- the bandit track and the boars -- and wear the better gear it finds
there, so an old bot looks and hunts differently from a new one. A player sees the
growth in `look <bot>` (what it wears, its role) and in where it hunts. Bots can't
be assessed, and that's fine: sizing up players waits for PvP.

The next project is more power 4-5 content for players (owner, 2026-10-18); the
grounds here should be shaped so a new zone gets bots by adding a table entry.

## Goals

- Two new grounds for hunters at **power 3 and up**: the bandit track and the boar
  wood.
- **The farms are for new players**: a hunter at power 3 or more no longer hunts
  them. A bot whose gear breaks and drops it below 3 goes back to the farms until
  it's mended, as it should.
- **Wear what's better.** After a fight a hunter compares what it looted with what's
  in that slot; strictly higher power is worn and the piece it replaced donated.
  Equal or lower is donated.
- **Same manners as the farms.** A bot leaves any ground a player is on, on foot
  (`yieldGround`), for `avoidFor`; it waits in Market Square when nothing's open;
  `consider` still gates every kill (nothing more than one power above itself).

## Non-goals

- **No bots in the Barrow or the mill.** Those are group fights (option 3, later).
- **No `keepOut` change.** `keepOut` is the wanderer's and explorer's; hunters walk
  hand-written routes and never read it. (The 2026-10-10 proposal assumed otherwise.)
- **No casting.** The hood grants assess, the cudgel smite, the jerkin provoke; a
  bot wears them for their power and never casts.
- **No chasing.** A bandit that flees takes its loot; the bot doesn't follow.
- **No atlas routing.** Grounds stay hand-written tables, walked by
  `TestGrounds_walk`.

## Anti-goals

- A bot reaching the wolves. They're power 5 and aggressive in the Deep Wood, south
  of the Edge of the Old Wood. A bot that grows to power 4 would `consider` them a
  fair fight, so **no route or patrol goes south from the Edge**, whatever the bot's
  power.
- A bot on the hedge-witch's floor, or hunting her: never prey (as today).
- Grown bots on the farms competing with new players.

## The grounds

Both start and end at the Waystone, like the farm grounds, so `route` is
`hollowfieldsRoute` and `homeward` works unchanged. The loops are my call; the
builder may reshape them as long as `TestGrounds_walk` walks them.

- **The bandit track** (power 3+): Waystone → east Hedgerow → south Wheat Field →
  south A Narrow Track → east The Bandit Camp → west A Narrow Track → north Wheat
  Field → west Farm Lane → north Waystone. Prey: bandit lookout (3, "lookout"),
  bandit (4, "thug" -- the lookout answers to "bandit" too, and a fled bandit can
  end up beside it). Loot: hood, cudgel.
- **The boar wood** (power 3+): Waystone → south Farm Lane → south The Old Bridge →
  south The Edge of the Old Wood → west The Woodcutter's Clearing → east Edge → north
  Old Bridge → north Farm Lane → north Waystone. Prey: wild boar (4). Loot: jerkin.
  Never south from the Edge (wolves), never south from the Clearing (the witch).

`prey.seen` is each mob's room line from mobs.json, as for the farms. Rats, geese,
beetles and dogs on these loops are not prey for these grounds.

Farm grounds: `maxPower` 5 → 2. New grounds: `minPower` 3, `maxPower` -- my call --
5, above which there's nothing in the Hollowfields worth a bot's time anyway.

## Safety test

`TestGrounds_safe` (replacing nothing; `TestKeepOut_safe` stays for wanderers): for
every ground, no aggressive or moonstruck mob of power above `minPower + 1` spawns in
any room of its route or patrol -- or anywhere in the zone, for one that wanders.
The farm grounds have `minPower` 0 (broken gear), but the moonstruck wild dog
(power 2) wanders the whole zone, so the test measures from `max(minPower, 1)` --
a new character's power, which `TestKeepOut_safe` measures from too -- and says so.
The angry goose (1, aggressive) is within that.

## Looting and wearing

`fight` already loots everything (`get all from corpse`). What's new:

- **What it found** is a table, like `kit`: each wearable a ground's prey drop, with
  its name as `inventory` and `equipment` show it, a keyword only it answers to among
  kit, starter gear and finds (the boar-hide jerkin's must not be "jerkin" -- the
  quilted jerkin answers to that; "boar" does), and the slot as `equipment` shows
  it. A test holds it to the real content, as `TestKit_theStoreSellsIt` does the kit.
- **When it decides** -- my call: on the donation trip, in the Donation Room before
  dropping anything. It's safe there, it already reads inventory and equipment
  nearby, and it batches. Read `inventory` (it shows each item's power) and
  `equipment`; for each found piece of strictly higher power than the slot's row
  (an empty slot counts as power 0; a broken row too, since it counts for nothing),
  `remove` what's there and `wear` the find. The best find per slot only.
- **What it donates**: the ground's loot keywords, the starter spares (as today),
  and the kit and finds keywords too -- `drop all.<kw>` leaves anything worn on, so
  this drops only what was taken off or not worn. Draughts it bought stay.
- **Repair**: `repairWord` answers a found piece's keyword as well as the kit's, so
  a worn hood goes to the smith like a worn cap instead of wearing out.
  `nextPiece` already skips a slot holding something it doesn't know.

## Consequences to expect

- **Growth is slow, on purpose.** Power is the average of what's worn, rounded down:
  the kit is six power-4 pieces and the power-1 token, 25/7 = 3. Power 4 takes 28,
  so three power-5 finds -- a drop is bumped +1 one time in ten (`LootPowerBump`).
  Days of play per bot, not hours.
- **The donation room gets power-4 hoods, cudgels and jerkins**, free to new
  players. The owner has said that's fine; the `economy` log's donation-room power
  will show it.
- **Coins**: bots on these grounds earn more (`coins_per_power`) and have nothing
  new to spend them on but repairs. Leave it; the coin-sink half of the 2026-10-10
  proposal is its own decision.

## Left to the builder

- Where the wear decision sits in code (a function beside `shop` in `outfit.go`, say)
  and how it's split for unit tests against canned `inventory`/`equipment` text.
- Whether `fight`'s kill count and `loot` need anything for a bandit that fled (it
  shouldn't count as a kill; check `deaths`).
- Docs: CLAUDE.md's hunter paragraphs ("its grounds are the farms", "tops out at
  power 3") and the comment above `grounds`; ROADMAP's bots line; LEVELS.md if it
  says where bots hunt.
