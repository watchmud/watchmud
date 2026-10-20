# The ruins of Emberfall

Status: **design, 2026-10-20.** The power 4-5 area agreed on 2026-10-18 ("bots first,
new areas next"), after hunters climbed the Hollowfields.

## Why

A player who has outgrown the bandits and the boars has nowhere of their own to go.
The Hollowfields top out at the wolves (5); the Drowned Mill (4-7) and the Sunken
Barrow (6-10) are small group fights of 7-8 rooms each. This adds the biggest place in
the game, meant for **one player at power 4-5**, with room to explore and loot worth
bringing home -- and, in a follow-up, room for bots to hunt.

## The story (owner's)

A mages' guild worked in the town, experimenting with harnessing primal energy --
fire, lightning. By accident they opened a portal to a demonic dimension, an
unleashing of terrible energies; orcs, daemons, wyverns and worse came through. The
guild closed the portal, but the explosion wrecked the town. What came through stayed.

## Goals

- **A ruined town east of the Bandit Camp** (owner's): the camp, a dead end today
  (its one exit is west), gains an east exit onto the road to the ruins.
- **Three parts** (owner's):
  - **the streets and ruined buildings** above ground, buildings you can go into;
  - **the sewers** beneath;
  - **the guild compound**: a large walled district of several buildings around an
    imposing, ruined **tower at its centre, sealed for now**.
- **Monsters from the portal**: orcs, daemons, wyverns, and others in that vein.
- **Loot** (owner's): **magical artifacts** -- the guild's gear -- and **daemonic
  components** (horns, ichor, scales...) that will matter for crafting later.
- **Artifacts use the existing abilities** (owner's): heal, smite, provoke, stun,
  ward, assess -- plus **one new signature ability, the fireball**, and the **bolt**
  (below).

## Non-goals

- **Crafting** (owner's: "later"). Components drop and are sellable; nothing uses them
  yet. Crafting is its own project.
- **The tower's inside.** The tower stands, is described, and its door doesn't open
  -- a locked door (my call), so the exits line shows it and `open` says it's locked.
  The loader wants a locked door's key to be an object: define the key, place it
  nowhere. Whatever opens the tower later drops it. A boss or the portal goes there.
- **Bot grounds in the town.** A follow-up in the hunters-climb shape (a table entry
  per ground, `TestGrounds_walk`/`_safe`/`_preyIsReal`) once the rooms exist.
- **Any player-targeted magic.** PvP is parked; nothing here touches a player but
  heal and ward, as today.
- **Group content.** Nothing here should need a group at 4-5. That's what the Mill
  and the Barrow are.

## Anti-goals

- **Danger spilling back into the Hollowfields.** Mobs never wander out of their own
  zone, so nothing from the ruins reaches the camp -- and the bandit-track bots
  patrol it. `TestGrounds_safe` keeps checking.
- **Wanderers and explorers walking in.** They're power 1. **`keepOut` gains east
  from the Bandit Camp**, or `TestKeepOut_safe` fails, as it should.
- **A fireball that's safe.** It's meant to be the dangerous thing you choose to do
  (below).

## The map

**Size** (my call): 30-40 rooms, a new zone of its own, roughly:

- **The approach** (my call): a few rooms of road east of the camp, scorched ground
  where the blast's reach begins, a broken town gate.
- **The town above ground**: streets, a square, and buildings to enter -- a tavern, a
  house or two, a shop -- most of them ruined. Orcs hold the streets.
- **The sewers**: reached by a grate or a collapsed cellar, darker, harder. Daemons.
- **The guild compound**: courtyards and buildings (my guesses: a library, dormitories,
  a workshop) inside a wall, around the sealed tower. Wyverns roost on its broken
  roofs. The hardest solo fights.

**Power** (my call): the manifest band is `{min 4, max 5}`. Streets at 4, sewers and
compound at 5, the approach at the bottom. `goals` will offer the town as the next
rung after the Hollowfields with no code change -- the band does it.

**Names**: the town is **Emberfall** (owner's), the zone "The Ruins of Emberfall".
Every room name unique in the world
(`TestRoomNames_uniqueInTheWorld`), and the zone must fit its grid
(`TestGrid_theContentFits`): the sewers sit a level down (`down`/`up`), so the
streets' grid and the sewers' share x/y.

**Aggression** (my call): mostly not aggressive, so a player picks their fights --
the streets as the Hollowfields are. A few aggressive things deeper in (sewers,
compound) where a player has chosen to be. Nothing aggressive in the approach.

## The monsters (my calls, for the builder to tune with `watchmud-sim`)

- **Orcs** (4): the streets. A scout, a warrior; maybe an orc that flees like the
  bandit, where running is in character.
- **Lesser daemons and imps** (4-5): the sewers, some aggressive.
- **Wyverns** (5): the compound's roofs and courtyards. The hardest single fight a
  solo 5 should take on.
- Anything else from the portal the builder likes, inside the band.

Tuned so a power-4 player in store kit plus a find or two wins against a 4 most of
the time and a 5 sometimes, measured with `cmd/watchmud-sim` (LEVELS.md).

## The loot

- **Daemonic components** -- a daemon's horn, ichor, wyvern scale, orc tusk (my picks):
  common drops, not wearable, sellable at the General Store -- so their category
  needs a price in economy.json, worth less than gear of the same power. Nothing uses them yet.
  Not `noSell`: they're the coin until crafting comes.
- **Guild artifacts**: gear dropped in the compound and by its monsters, rarer, each
  granting an existing ability -- a warded robe, a wand of bolts, a lens of assess,
  and so on. At the mob's power plus the loot bump, as all loot is.
- **The fireball staff**: the one item that grants fireball. Rare -- the prize of the
  compound.
- **No `noSell`/`noDonate` drops** (my call): those are for a boss's own, and there's
  no boss until the tower opens.

## The bolt (Go, small)

The guild's directed energy: fire or lightning at one foe. **Mechanically it's
smite** -- a blow that always lands, the cost being mana and a cooldown. It's its own
ability id, `bolt`, with its own numbers in abilities.json and its own words ("You
hurl a bolt of fire at the orc"), so smite keeps its words on weapons. In `effects`,
`bolt` reuses smite's effect; it needs its own event or a field on `Smote` saying
which it was -- the builder's call -- and a render case. Granted by the guild's wands
and staves.

## The fireball (Go, the new mechanic)

The guild's signature (owner's): **fire to every foe in the room, and every one of
them comes for you** (owner's: "the first -- it's dangerous").

- **Target**: a new target kind, `room` (name the builder's), resolving to every
  mob in the caster's room that could be `kill`ed -- skipping `nofight` and
  can't-fight mobs (the shopkeeper, the janitor). Summons are fought, so they're hit. **Refused in a no-fight room**, as smite is; **no mob there to hit**
  is a refusal before anything is spent (like `NO_FOE`).
- **Damage**: each mob takes `amount.For(power)`, the same for each, no to-hit roll,
  lower per target than smite (my call, to tune: base 4, per_power 2 -- 14 a mob at
  power 5, against smite's 23). Each death goes through
  `combatantDied` in turn, loot and all.
- **Everyone it hits is drawn in**: through `openFight`, as smite does -- a caster not
  yet fighting opens a fight with the first and is turned on by the rest; a mob
  already fighting someone else (a groupmate) is turned on the caster only if it was
  fighting nobody. Any scripted mob's opener fires as usual. **No player is ever hit,**
  groupmates included.
- **Wear**: a point off the staff per cast, not per mob hit (my call).
- **Cost**: high mana, long cooldown (my call: around 40 mana, 60s).
- **Events and words**: one event for the cast the room sees, and per-target damage
  the caster and room read; the builder decides whether that's one event carrying a
  list or one per mob. Message order follows smite's: the hits, then wear, then the
  deaths.
- **sim**: melee only today; say so, as it already does for abilities.

## Order of the work (my call)

Each step shippable on master on its own:

1. **The zone and the join**: rooms, the camp's east exit, the sealed tower,
   `keepOut`, manifest band. Walkable, empty.
2. **Mobs and loot**: monsters, components, artifacts granting existing abilities,
   instructions.json, tuned with the sim.
3. **Bolt**: the ability, its words, the wands.
4. **Fireball**: the target kind, the effect, the staff in a loot table.
5. Docs: CLAUDE.md, ROADMAP, `docs/working.md`, `site/` if it names areas, the
   socialite's `faq` if it says where to go next.

The owner writes the Go (3, 4, and any loader or engine change step 1-2 turns out to
need); Claude writes the content, tests, Lua and docs.

## Left to the builder

- Every room, description and exact layout; the mob roster and numbers inside the
  band; loot chances.
- Whether bolt and fireball get their own events or extend `Smote`.
- What the socialite tells a newcomer about the town.
