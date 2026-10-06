# These Are The Rules

## Armor

Describes the AC bonuses for each type of armor depending on which slot it's in.

The same number does double duty: it is the AC the piece adds, and it is what the
piece contributes to any role marked `from_armor` in roles.json. Armor that protects
you more is armor that argues harder that you're the one standing in front, so there
is nothing to keep in sync.

AC starts at 10 (`rules.BaseArmorClass`) and a hit is a d20 that *meets or beats* it,
so 10 is unarmored and hit 55% of the time. A mob's `"ac"` in mobs.json is on that same
absolute scale -- 10 is an unarmored creature, not a bonus on top of one -- and a mob
that doesn't name one gets 10. Writing `"ac": 0` means a creature that cannot be missed,
since a d20 never rolls below 1.

## Roles

This is the replacement for classes: your role is determined by what gear you're wearing.
Healer Gear = you're a healer, etc.

A role with `"from_armor": true` is one that armor argues for on its own, using the
table above -- that's Tank. Everything else says what it's worth by hand, with a
`"roles"` key on the object: a knife has no armor type to derive anything from.

## Abilities

What gear lets a player *do*: `abilities.json`, one entry per ability.

```json
{ "id": "heal", "name": "heal", "mana": 20, "cooldown": "10s",
  "target": "friend", "amount": { "base": 10, "per_power": 2 } }
```

- `id` is what objects grant and what players type (`cast heal`); `name` is what
  `abilities` shows.
- `mana` is the cost, and `cooldown` a duration ("10s", "1m") before it can be cast
  again. Everyone has 100 mana; it comes back a little every few seconds, even in a
  fight.
- `target` is who it can be aimed at: `self`, `friend` (yourself, or a player in the
  same room), `foe` (a mob in the room you could fight), `mob` (any mob in the room -- for looking,
not fighting) or `none`.
- The rest are the ability's own numbers. Heal's `amount` is `base + per_power x
  power`, where power is the power of the item granting it -- a power-3 censer heals
  16. Smite, the first `foe` ability, uses the same `amount` for its damage: a power-1
  cudgel hits for 11, and it always lands.
- `duration` is how long what an ability leaves behind lasts. Only ward has one: its
  `amount` is the damage the shield takes before health does, and it fades after
  `duration` ("30s") if it isn't used up first. Casting it again refreshes it to the
  larger of what's left and what's new; it never stacks.

An object grants abilities while it's worn and unbroken, with an `"abilities"` key in
its zone's objects.json:

```json
"abilities": [ "heal" ]
```

Two items granting the same ability don't stack: the stronger one is what you cast
with. An object naming an ability that isn't in this file fails startup, and so does an
ability here that the engine has no code for -- what an ability *does* is Go
(`world/abilities.go`), so a new one is a code change as well as an entry here.

The file is optional: no `abilities.json` means nobody can cast anything.

## Species

This is the definition of species and lineage, which replaces 'Race' and 'Class' and
is now mostly cosmetic. Your choice of lineage doesn't dictate any stats, just how
you feel like role-playing.

## Durability

`durability.json` says what gear starts out able to take. Plate outlasts leather outlasts
cloth; anything with no armor type -- a knife, a censer -- takes `default`.

```json
{
  "default": 40,
  "on_death_percent": 10,
  "armor": { "cloth": 20, "leather": 40, "plate": 80 }
}
```

An object can override it with its own `"durability"` in objects.json, for the one blade
that deserves to be special. `"durability": 0` means a thing that never wears out, which
is also what every object gets if there is no durability.json at all -- an absent file is
durability switched off, not a world full of gear that starts out broken.

Two things wear gear out. A landed blow in combat costs one point off one piece of the
defender's armor, chosen at random, and one point off the attacker's weapon; a miss costs
nothing. **Dying** costs `on_death_percent` of what *every* piece you died in started at,
so dying in plate and dying in a wool tunic cost the same number of deaths rather than the
same number of points. Always at least one point, so cheap gear isn't immortal. Zero, or
saying nothing, makes dying free. At zero a piece is
**broken**: still worn, still carried, and worth nothing -- no AC, no argument for any
role -- so the breastplate that gives out mid-fight is a player watching their armor class
drop and, if it was carrying them, their role change with it. Nothing repairs gear yet.

## Starting Gear

What a brand-new character is created holding: `starting_gear.json`, a list of object
definitions named by the zone that defines them, in the order they should arrive.

```json
{ "zone": "wrathrock", "object": "training_dagger", "equip": true }
```

`"equip": true` means the character starts with it worn; leave it off and the item is
only carried. There is no slot here on purpose -- the object definition already names
the one slot it goes in, and a second copy of that would only be something to keep in
step. An item marked `equip` that isn't wearable, two items claiming the same slot, or
an object or zone that doesn't exist, all fail startup rather than quietly handing out
less than the file says.

The file is optional: no `starting_gear.json` means new characters start with nothing.

Nothing in here says what role the kit adds up to, because nothing can -- a role is read
off the equipment at the moment it's asked for. The wrathrock kit (a training dagger, a
wool tunic and a cloak, plus a waterskin) makes a level 1 character a Striker with AC 11,
and stops mattering the moment they wear something else.
