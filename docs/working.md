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

## Where things stand (2026-10-17)

Released: **v0.12.0** -- `goals` and `map`; screen-reader mode and MTTS; MCCP2, MSSP
and GMCP `Comm.Channel.Text`; boss drops that can't be sold or donated, `help rules`
and the `economy` log; the General Store's power-4 kit and strong draught, and hunter
bots that buy the kit and keep it repaired. Earlier: the Drowned Mill, doors and
chests, bags, potions, groups, positions, ooc and socials, the janitor and crows
(v0.11.0), provoke, stun, ward, assess, GMCP (v0.10.0), nohassle, slay, smite
(v0.9.0), heal and mana (v0.8.0), the economy (v0.7.0), bots on foot and NAWS
(v0.6.0), color and repair (v0.5.0), the inhabitant bots (v0.4.0).

Shipped but not announced on the site, on purpose:
- Screen-reader mode wants a blind player to try it before it's announced.
- `Comm.Channel.Text` is read by Mudlet 5's starter chat pane; `watchmud.lua`
  doesn't use it yet.
- MSSP is on in deploy/app.yaml; submitting the game to the listings is the owner's
  step.
- `help rules` is a draft automation policy, to be reworded.

The Mudlet add-on works in a real Mudlet: 5.0.1, against v0.12.0, on 2026-10-17 --
`Room.Info` and `Char.Vitals` arrive and the map draws. Mudlet 5 also builds a
starter UI of its own (a dock on the right: map, chat, vitals, fed by GMCP) whose
map stays empty without the add-on; how the two share the screen isn't settled.
Whether that Mudlet took up MCCP2 wasn't checked; text arrived fine either way.

**On master, not yet released:**
- The welcome banner always comes before the name question (it could lose a race,
  and a bot waiting for the banner then missed the question). Nothing for the site.

**Next, roughly:** tuning the mill and the King once players have tried them; the bots
ROADMAP lists (explore-and-map, bots
that wear what they find). The Barrow-King's AC waits for a real group to try him.
Leftovers: `watchmud.games` DNS points nowhere. (An unhealthy game now fails the
`uptime` workflow, once it is on master.)
