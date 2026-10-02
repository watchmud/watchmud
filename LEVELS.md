# Levels: power comes from gear

The progression loop: kill things near your power, they drop gear a little above it,
the gear raises your power, and bigger things become worth fighting. Crafting joins
later as another place gear comes from.

The model is Destiny's Power level (Diablo IV's Item Power and WoW's item level are
cousins of it): **your level is what you are wearing.** It follows from rules
CLAUDE.md already sets -- the gear is the truth, don't route mechanics through a
hidden number, derive combat stats from equipment.

## Decided

- **No XP, and no stored level.** Nothing earns points, and nothing on `player.Player`
  or `player.Record` holds a level. A cosmetic rank for `who` could come someday;
  nothing mechanical may read it.
- **Power is per item instance**, not per definition. The same rusty sword is power 5
  off a goblin and power 15 off an ogre, so one definition serves every tier. Like
  durability, it is set wherever the instance is made (the mob that dropped it, the
  zone that reset it, starting gear, later crafting) and saved as a `*int` in
  `InventoryRecord`, so records from before power existed still load.
- **A player's power is derived on every read, like role.** It's the average power of
  the equipped, unbroken items; **empty slots don't count.** Nothing equipped is
  power 0. No `Power` field on `Player`, for the same reason there is no `Role`
  field: a stored copy can disagree with the slots.
- **AC stays as it is**: armor type by slot, from `armor.json`. Plate means *harder to
  hit*; power means *stronger*. Two things, tuned separately.
- **Mobs get `"power"` in mobs.json**, defaulted by the loader the same way `ac` is.
  Zones get a power band in the manifest, which is how content says "this is the
  10-15 area". A mob with no `"power"` is the bottom of its zone's band;
  an explicit one may sit outside it, which is what a boss is. A zone with no
  band is 0-0.
- **Combat reads the power *difference*, clamped.** d20 vs AC has to stay bounded --
  if AC grew with power, by power 30 nothing would ever connect. So hit and damage
  stay as they are, and `attacker.Power - defender.Power` becomes a modifier on top.
  It's arithmetic, not a cutoff like role, so it doesn't break the rule against
  branching on a role.
- **Loot comes from loot tables on mobs**, goes into the corpse, and drops at the
  mob's power with a small chance of a bit more.
- **`consider <mob>`** compares powers. Considering a *player* is tabled.

## Tuning placeholders

Every number here is a guess until playtesting. They start as named constants in
`rules/`, each with a comment pointing back here, and move into a `rules/*.json`
together once there's something to tune against. Keep them in one place so that
move is one change.

| What | Placeholder | Where it matters |
|---|---|---|
| Regen interval | 5s (`mudtime.json`) | how long a grind stalls between fights |
| Regen amount | 5% of max health, at least 1 | same |
| Mana regen | 5% of max mana, at least 1, fighting or not (`rules.ManaRegenPercent`) | how many heals a long fight affords |
| Max mana | 100, flat (`rules.MaxMana`) | the budget a healer walks into a fight with |
| Dropped decay | 30m (`rules.DroppedDecay`) | how long a donation waits for a newbie |
| Bare hands damage | 1d2 (`rules.BareHands`) | also a mob with no `"damage"` |
| Averaging worn power | rounds down | how soon one upgrade shows in your number |
| Power delta clamp | ±10 | how far out of your league anything can be |
| To-hit per point of delta | +½ | whether a higher mob can be hit at all |
| Damage per point of delta | ±5%, rounded, a hit does at least 1 | how long fights above your power take |
| Loot power bump | 10% chance of +1, 2% of +2 | **the pace of the whole game** |
| Corpse decay | 5 minutes (`rules.CorpseDecay`) | time to loot vs. rooms filling up |

## Phases

Each one leaves `make check` green and is playable on its own.

### 1. Prerequisites -- done

- **Regeneration.** Health comes back on a `regen` pulse for anyone not fighting,
  players and mobs both. Without it, dying (which leaves you on 1 hp) ends your
  progress for good, and so does a bad fight.
- **Weapon damage from the weapon.** `Player.WeaponDamageRoll` is a hardcoded `1d6`.
  Give `object.Definition` a damage roll from objects.json and read it from whatever
  is in `SlotWield`, bare hands otherwise.

Damage *type* waits. `HasResistanceTo` and `IsVulnerableTo` both return false for
everything, so a weapon's type would change nothing yet, and `combat.DamageType`
has no parser to read it from JSON. It belongs with the first resistance.

*Done when:* a hurt player heals back while idle, and swapping weapons changes the
damage.

### 2. Power -- done

- `object.Instance.Power`, saved with the record the way durability is.
- `Equipment.Power()`: the average over equipped, unbroken items.
- `mobile.Definition.Power` from mobs.json, and a power band per zone in the manifest.
- The combat modifier from the clamped delta, in `combat/` against a new
  `Combatant.Power()`.
- Power shown in `stat` and `equipment`; `consider <mob>`.

*Done when:* the same fight goes differently as your gear gets better, and `consider`
says so before you start it.

### 3. Loot -- done

- Loot tables in mobs.json: `"loot": [{"object": "wrathrock/rusty_sword", "chance": 25}]`.
- Corpses hold the drops, which finishes the `TODO transfer m's possessions` in
  `world/corpse.go`. Drop power is the mob's, with the bump from the table above.
- `get <item> from <container>`, and `get all from corpse`.
- Corpses decay, or rooms fill up with them.

*Done when:* killing something gives you something worth wearing.

**The first loot tables** are already decided, for the Hollowfields and the Sunken
Barrow. Every object below is defined in that zone's objects.json and placed nowhere,
waiting for these. Chances are placeholders like everything else here.

| Mob | Drops |
|---|---|
| field rat | rat pelt (50) |
| angry goose | goose feather (60) |
| giant beetle | beetle carapace (50) |
| wild dog, wolf | rat pelt (30) -- until there is a proper hide |
| bandit lookout | bandit hood (20) |
| bandit | bandit hood (15), bandit cudgel (15) |
| wild boar | boar-hide jerkin (20) |
| hedge-witch | ring of mending (15), sprig censer (15) |
| grave rat | grave dust (40) |
| barrow skeleton | barrow helm (15), barrow plate (10) |
| ghoul | bone charm (20), grave dust (40) |
| the Barrow-King | barrow blade (35), black iron crown (35), barrow plate (35) |

The hedge-witch is where the first healers come from: the ring and the censer are what
make someone a Healer, and they have to be found in newbie country or no group ever has
one to take into the barrow.

**Out-level it and it isn't worth it, on purpose.** Drops come out at the *mob's* power,
never the killer's, so the King's gear is power 15 whoever takes it. A power-20 player
who solos him is trading down; the loot is only worth having to the people who needed
a group to get it. Don't "fix" that by scaling drops to the player.

### 4. Content

Power bands on the zones and mobs to fill them. Content work, not code, and where the
placeholders above get replaced.

Started: **the Hollowfields** (1-5, south of Wrathrock -- farms, bandits, an old wood)
and **the Sunken Barrow** (6-10, down under the overturned oak in the wood's far corner),
whose Barrow-King (power 15) is the first boss.

**The Barrow-King is a group fight, and it takes all three kinds of gear.** Nothing may
branch on a role, so each has to be something the gear actually does:

- *tankish* -- plate's AC. He has to hit hard enough that anyone else in front of him
  dies fast, and plate has to turn enough of it aside to survive with healing.
- *healish* -- the heal that the ring of mending or the censer grants. Without it even
  the tank runs out before he does.
- *damageish* -- weapon dice. His health is big enough that a tank and a healer on
  their own can't finish him.

Targets to check him against once power modifiers, heals and threat all exist:

- solo, newbie kit: dead in about 20 rounds, having done almost nothing to him
- tank plus damage, no healer: the tank dies before he's at half
- tank, healer, damage: they win, and the healer is busy the whole fight

Until then he is unbeatable, and that's fine: he's the reason heals get built.

**With power in combat, AC 16 may be too much.** Five or more below him is -5 to hit,
so anyone at power 5 or under can never land a blow (20 - 5 < 16), and a group at the
barrow's top end, power 10, needs an 18 -- 15% a swing, for 75% damage. Out-of-league
is the point, but that may be past "needs a group" and into "needs nothing below 12".
Decide when there's a group to try it: lower his AC to 13 or 14, or accept that the
barrow's band is the gear you farm *before* you try him.

Needed for that fight, and not yet anywhere else on this page:

- **Abilities from gear** (below, under Tabled -- no longer tabled): `cast heal`, and
  whatever resource or cooldown limits it.
- ~~**Retargeting.**~~ Done. A mob keeps whoever engaged it first, and when they die or
  flee it turns on the earliest remaining attacker, so the King no longer stands still
  once the tank falls. (It used to: two newbies could beat him by taking turns dying.)
- **Threat**, later: a way for a tank to take him *back* once he has turned on someone
  else. First-engaged-holds covers the opening; nothing covers a pull gone wrong.

### 5. Crafting (later)

Another place an item's power comes from, so it slots into the same model:

- **salvage**: break gear down into materials, at that gear's tier
- **craft**: make gear, at the materials' tier
- **infuse**: feed a stronger item into a favourite to raise its power (Destiny's trick)
- **repair**: the durability sink. Done 2026-10-01: `repair` at Wrathrock's smithy, for
  coins (content/rules/economy.json).

## Tabled

- **Considering a player** -- their gear, role, power. PvP isn't on the board.
- **Abilities from gear**: a ring of healing gives a healing cast, a mace of smackdown
  gives a big hit with a stun. It's the natural next step after power: gear decides
  what you can *do*, as well as how strong you are.

## Known risk in "average only what you wear"

Wearing one power-20 ring and nothing else makes you power 20, the same as a full
power-20 kit. That's fine while power only shifts combat numbers a little, and it may
matter less once abilities come from gear. If it becomes a problem, two options that
keep the "empty slots don't count" feel:

- **weight by slot**: the weapon and body count for more than a ring
- **split by side**: offense from the weapon's power, defense from the armor's average
