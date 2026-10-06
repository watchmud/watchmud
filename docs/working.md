# Working on watchmud

How this project is worked on, and where things stand -- the notes a fresh session
(local or cloud) would otherwise not have. CLAUDE.md is how the code works; ROADMAP.md
and LEVELS.md are the plan of record; this is the rest.

## How work goes

- **Momentum.** Once a direction is agreed, keep going step after step without
  re-confirming. Each step is its own small commit on master, `make check` green first.
  At the end of a step, name the next one.
- **Feel and tuning calls aren't decided silently.** Do the work, then raise the choice
  in one line with a recommendation (a boss's AC, a cooldown, a drop rate).
- **Decisions go in the docs**, not only in chat: LEVELS.md for game design, ROADMAP.md
  for the plan and known problems, CLAUDE.md for how the code works.
- **Bigger features** get a short spec in `docs/superpowers/specs/` and a plan in
  `docs/superpowers/plans/` before code; small ones a design agreed in chat.
- **Releases are batched.** Master accumulates until there's a lot worth shipping, and
  the owner asks for a release. Don't offer one after every fix.
- **Don't trust a "not yet" in the docs without checking the code**; they've lagged
  before.

## Production

- Live at **watchmud.com**: telnet on 4000, TLS on 4443. The guide at
  **www.watchmud.com** is GitHub Pages from `site/`.
- Production runs only **version-tagged images** (`vX.Y.Z`, tagged on a `release/` or
  `hotfix/` branch, built by `.github/workflows/build.yaml`). Master is development;
  pushing it builds a `:master` image and changes nothing live.
- **Deploying is the owner's step**, run on the droplet with `deploy/deploy.sh vX.Y.Z`
  (runbook: `deploy/README.md`). No session has access to the box: nothing is pulled
  or built there by hand, and every deploy disconnects everyone.
- **Never create characters on production to test**: names are permanent.
- **A `site/` change describing unreleased features waits** to be pushed with its
  release; pushing `site/` to master publishes it immediately.

## Where things stand (2026-10-06)

Released and deployed: **v0.10.0** -- provoke, stun, ward and assess; the Barrow-King's
summons; recall as a trinket; `wear`/`wield` of `2.thing`; wizard `gold`; `/healthz`
and the compose healthcheck; `who` as a table with bots apart; the wanderer and
socialite bots and "entered the game for the first time"; GMCP `Char.Vitals` and
`Room.Info`. Earlier: nohassle, slay, smite (v0.9.0), heal and mana (v0.8.0), the
economy (v0.7.0), bots on foot and NAWS (v0.6.0), color and repair (v0.5.0), the
inhabitant bots (v0.4.0).

**On master, not yet released** -- each needs its line in the site guide when it ships:

- `abilities`, `equipment`, `inventory`, `stat`, `role` and `list` are aligned tables
  in color; `equipment` no longer prints instance ids; `inventory` shows power and
  condition and folds duplicates; `stat` adds mana, armor class and the room by name
- The Mudlet add-on (`mudlet/`): bars and a map from GMCP. Its site section is in this
  branch already -- it needs only v0.10.0's GMCP, so it goes live with the merge, not a
  release. Nobody has run it in a real Mudlet yet: try it before telling players
- The Drowned Mill (power 4-7), west of the Millpond, with the Drowned Miller (power 8);
  six new items; the socialite knows where to send players who've outgrown the farms
- Doors, locks and keys: `open`/`close`/`lock`/`unlock`; "(closed)" in the exits line.
  The first is the mill's iron grate, locked, in front of the miller; the rusted key
  drops from drowned millhands (25%)
- Chests: `"container"` objects with their own lid and lock -- the mill's grain bin (the
  strongbox key inside) and the miller's strongbox. Zone resets now top objects up to
  `instance_max` instead of adding one more each time: the Sample Zone's fountains have
  been piling up in production since it launched
- `put <item> in <chest>` (and coins): get-from backwards
- Bags: a leather satchel (holds 10) at the General Store; `put`/`get from`/`look in`
  work on one you carry, and what's in it is saved with you. Bags don't nest, coins stay
  in the purse, and `inventory` shows how full each one is

**Next, roughly:** tuning the mill and the King once players have tried them; the bots
ROADMAP lists (explore-and-map, bots
that wear what they find). The Barrow-King's AC waits for a real group to try him.
Leftovers: `watchmud.games` DNS points nowhere; nothing alerts on an unhealthy game.
