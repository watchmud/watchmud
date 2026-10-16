# Bots that buy their kit

Status: **agreed 2026-10-15**, option 1 of
`2026-10-10-bots-that-grow-and-coin-sinks-design.md`, against the power-4 stock the
General Store got the same day (98e8467).

## Why

A hunter takes coins off every corpse and never spends one. With this, it spends them
the way a player would: on the General Store's power-4 kit, piece by piece, and on the
smith keeping it in repair. Two things come of it:

- **A coin sink** for the coins bots hold, which the `economy` log counts apart.
- **A visible difference** between a bot that has been out a while and a new one:
  `look <bot>` shows what it wears.

## What doesn't change

The grounds, `consider`'s rule (nothing more than a power above itself), `keepOut`,
`TestKeepOut_safe`. A full store kit with the power-1 temple token is power 3 -- a
farm hunter still, a safer one. Only hunters do it: wanderers and explorers loot
nothing, and a socialite never leaves Temple Square.

A bot still reads only what a player reads (CLAUDE.md, "The bot is a player"): the
equipment listing, the purse line under `inventory`, and what the store and the smith
say. No prices or durabilities are written into the bot.

## The kit

A hand-written table in `bot/outfit.go`, like the grounds, bought in this order:

| Piece | Slot | Replaces |
|---|---|---|
| arming sword | wield | training dagger |
| quilted jerkin | body | novice tunic |
| studded cap | head | -- |
| studded boots | feet | -- |
| riding gloves | hands | -- |
| oilskin cloak | about body | travelers cloak |

Power, averaged and rounded down over everything worn: 1, then 2 after the jerkin, 3
after the gloves. Each entry names the piece by its full name (`buy` and `wear` take
an object's name or an alias; "cap" alone would buy the power-2 leather cap, which is
earlier in the stock), and what it replaces by an alias unique to it ("training",
"novice", "travelers") -- never "cloak", which the oilskin answers to too.

## The visit

`town` already asks for `equipment`. It now reads its rows -- slot, item, power,
condition -- and asks for `inventory` for the purse. Then, before picking a ground,
it goes on an **outfit** visit if either:

- **something needs repair**: a worn piece reads `broken`, or has a third or less of
  its durability left (`n/m` with `3n <= m`, the mark that turns yellow for a player);
- **the next piece is affordable**: the first piece on the list not worn, and the
  purse covers what it's known to cost -- or its price isn't known yet.

The visit, a new state between `town` and `travel`:

1. To Market Square (a step south of Temple Square, where every route starts).
2. If something needs repair: west to the Smithy, `repair all`, back east. The cost
   is proportional to wear, so repairing at the yellow mark costs no more over time
   than repairing every visit. Short of coins, the smith mends what it can in order
   and says `... would cost N coins, and you have M.` for the rest; that's fine.
3. If the next piece is affordable: east to the General Store, `buy <name>`. On
   `You buy <item> for N coins.`: `remove <replaced>` if the slot is taken, then
   `wear <name>`. On `<Item> would cost N coins, and you have M.`: nothing bought.
   Either way N is remembered as that piece's price. Bought, it goes on down the list
   while the purse covers the next known price; refused, it stops. Back west.
4. On to `travel`, which already carries on a route from partway down it.

**Prices are learned, not known**: from the two store lines above, in memory, per
piece. A bot that doesn't know a price goes and asks once by trying to buy it; after
that it goes back only when the purse covers it. A reconnect forgets, and asks again.

**What's removed is donated.** The starter piece stays in the pack, and the donation
trip sends `drop all.<alias>` for every starter alias on the list, as well as the
ground's loot -- so a piece removed before a reconnect goes too. Nothing worn is
dropped: `drop` leaves worn things on. A power-1 tunic in the donation room is no gift
above the farms, and decays like anything else dropped.

**What goes wrong.** An answer it didn't expect is `errLost`, as everywhere else: it
recalls and starts over. Attacked on the way is `fightThen`, as `donate` does -- unlikely
in town, but the rule is the bot's, not the town's.

## Tests

- **Route**: Temple Square -> Market Square -> Smithy -> Market Square -> General
  Store -> Market Square, walked against the real content, as `TestGrounds_walk`
  walks the grounds.
- **Kit**: every piece on the list is something the General Store sells, under that
  name, and every replaced alias is on the starting kit -- against the real content,
  so a renamed object fails `make check`.
- **Reading**: the equipment rows (a slot of two words, a piece with no durability, a
  broken one, a worn one), the purse line, and the bought and too-expensive lines,
  as text the renderer really produces.
- **In the game** (`adventurer_game_test.go`, real content in-process): a hunter whose
  record starts with 200 coins buys the sword and the jerkin, wears them, and is power
  2 on `equipment`; with too few coins, it learns the price and goes hunting. One with
  a worn piece goes to the smith first. `startGame` grows to hand back its store, so a
  test can set a record's coins while the character is logged out.

## Not in this

Buying draughts; wearing what drops (option 2); bots past the farms; a bot spending
on anything but its own kit. If the `economy` log shows bots still piling up coins
with full kits, that's the next question, not this one's.
