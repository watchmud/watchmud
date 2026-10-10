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

## Where things stand (2026-10-07)

Released: **v0.11.0** -- the Drowned Mill and its miller; doors, locks, keys and
chests; bags, `put` and `give`; `look <thing>` at last; potions; groups, follow and
assist; positions; the ooc channel, socials, emote, reply, whisper and ask; `track`,
`wimpy`, `split`, `where`, `time` and the moon over Seattle; `junk` and `donate`; the
janitor, the crows, the shopkeeper and fleeing bandits; player reports; the wizard
toolkit and moderation; lists as aligned tables; the explorer bot, the load test and
the fight simulator; the Mudlet add-on. Earlier: provoke, stun, ward, assess, GMCP
(v0.10.0), nohassle, slay, smite (v0.9.0), heal and mana (v0.8.0), the economy
(v0.7.0), bots on foot and NAWS (v0.6.0), color and repair (v0.5.0), the inhabitant
bots (v0.4.0).

The Mudlet add-on still hasn't been run in a real Mudlet: try it before telling players.

**On master, not yet released** -- each needs its line in the site guide when it ships:
- `goals`: the zones your power fits, the next one, and the walk to each.
- `map`: the rooms around you, drawn.
- `screenreader [on|off]`, and MTTS: a client that says it reads to a screen reader
  gets words instead of symbols. Wants a blind player to try it before it's announced.
- Boss-only drops can't be sold or donated; `help rules` (a draft automation policy,
  to be reworded); an `economy` log line every 15 minutes.
- MSSP, for listing sites: on in deploy/app.yaml. Once released, submitting the game
  to the listings is the owner's step.

**Next, roughly:** tuning the mill and the King once players have tried them; the bots
ROADMAP lists (explore-and-map, bots
that wear what they find). The Barrow-King's AC waits for a real group to try him.
Leftovers: `watchmud.games` DNS points nowhere. (An unhealthy game now fails the
`uptime` workflow, once it is on master.)
