# Bots that grow, and where coins go

Status: **proposal, 2026-10-10, for the owner to decide.** Not started: both are
feel and tuning calls (docs/working.md), and they meet in the same place -- the
economy ROADMAP's "Economy inflation" item worries about.

## Where things stand

- **Coins come in** from every kill (`coins_per_power` 3 a power) and every shop sale
  (40%). **They go out** only to the General Store and the smith. The store's stock is
  power 1-2, so a player past the farms has nothing to buy but draughts and repairs.
- **Bots** take coins off every corpse and never spend one. Since today, the
  `economy` log line counts their coins apart, so they don't hide what players do.
- A hunter bot is power 1 for good and fights nothing above power 2 (`bot/grounds.go`).
  Its prey drop pelts, feathers and carapaces -- nothing it could wear -- so "bots that
  wear the upgrades they find" has nothing to find on its current grounds.

## Bots that grow

The roadmap's line is "bots that **wear the upgrades they find** and grow into the
Barrow". Three ways to get there, cheapest first:

1. **Buy, don't find.** A hunter with coins walks to the General Store and buys the
   power-2 sword, cap, boots and gloves, wears them, and is power 2 for good. It
   spends coins it would otherwise hoard (a sink, of a sort), and stays on the farms,
   just safer. Cheap: the shop exists; the bot needs a `shop` state and `buy`/`wear`
   patterns. Grows a step, not into the Barrow.
2. **Wear what drops on a wider ground.** Add grounds for power 3-5 (the bandit track,
   the boars) gated by the bot's own `equipment` power, and have `fight` loot wearables
   too, comparing a drop's power (`look <item>` prints it) with what's worn. The bot
   then climbs the Hollowfields on its own. Needs: the wear-if-better decision, a
   `keepOut` that opens as power rises (today it's a safety list for power 1), and
   `TestKeepOut_safe` reworked per power band. A real feature: a spec and plan first.
3. **Grow into the Barrow.** (2) plus groups of bots, since the King is a group fight.
   Bots grouping, assisting, healing each other -- the most work, and the most
   interesting to watch. Later.

**Questions:** Should bots compete with players for drops above the farms at all?
Today the rule is "a bot leaves a ground a player is on"; a growing bot takes gear a
player might have had, and then donates it -- which is the very inflation ROADMAP
names. If they grow, should they **keep** what they wear and donate only what's below
their own power? (Recommendation: (1) now, as a sink and a visible difference between
an old bot and a new one; (2) only with "never donate above the farms' band".)

## Coin sinks

Candidates, each a small change; none is built:

- **Store stock that climbs.** Power 4-6 pieces at the General Store (or a second shop
  in a later zone), priced by the existing economy table. Coins buy a step up, never
  past what the Mill drops. The simplest sink, and it also gives (1) above its point.
- **Recall costs a coin or two** -- no: recall is how a lost new player gets home, and
  the token is deliberately free. Listed so it isn't proposed again.
- **Repair by power** is already a sink, and grows with the player. Raising
  `repair_percent` would make it bite; 50% is tuned against `sell_percent` 40 so
  nothing loops, and that stays true for any repair above 40.
- **Housing, a bank fee, rent.** Classic, and each wants a feature first. No.
- **Consumables worth buying at every level**: draughts at higher power (the stock
  table already names the power), scrolls once there are abilities to put on them.

Recommendation: store stock at power 4-5 plus healing draughts at power 5, watched
through the `economy` log for a couple of weeks before anything else.
