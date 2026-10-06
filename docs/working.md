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

## Where things stand (2026-10-05)

Released and deployed: **v0.9.0** (nohassle, slay, smite). Earlier: heal and mana
(v0.8.0), the economy (v0.7.0), bots on foot and NAWS (v0.6.0), color and repair
(v0.5.0), the inhabitant bots (v0.4.0).

**On master, not yet released** -- each needs its line in the site guide when it ships:

- `cast provoke` (tank: turn a mob onto yourself) -- barrow plate and helm, boar-hide jerkin
- `cast stun` (a mob skips two swings) -- the mace of smackdown, from barrow skeletons
- The Barrow-King calls up two skeletons at half health, after a two-second pause
  (Lua `me:summon` and `wait()`)
- Recall as a trinket: the temple token, worn on the neck, 60s cooldown; every
  existing character is handed one at their next login
- `wear` and `wield` understand `2.thing` and pass over what can't go on; both name
  what they put on
- Wizard `gold <amount>`
- `cast ward` (a shield on yourself or a friend that takes damage before health does)
  -- the ring of mending, from the hedge-witch
- `cast assess` (a mob's exact health, AC, power, damage, target and stun) -- the
  bandit hood
- `/healthz` and a compose healthcheck; `deploy.sh` waits for healthy before the smoke
  test (no site change -- it isn't something players see)
- `who` is a table, in color, with the bots in their own section at the bottom; the
  `[bot]` tag is gone, so the site's "Anyone marked `[bot]` in `who`" changes with it
- Wanderer bots (`WATCHMUD_WANDERERS` in deploy/.env): roam the safe parts of the
  world, linger, fight only back, take nothing. Needs characters made and flagged,
  like the hunters (deploy/README.md, "Bots")
- A socialite bot (`WATCHMUD_SOCIALITES`): stands in Temple Square, welcomes each new
  character, answers questions said or told. The room now hears "X has entered the
  game for the first time." for a brand-new character

**Next, roughly:** a release of the above; content; the bots ROADMAP lists (explore-and-map, bots that wear what they find). The Barrow-King's AC waits for a real group to try him.
Leftovers: `watchmud.games` DNS points nowhere; nothing alerts on an unhealthy game.
