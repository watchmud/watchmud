# Abilities from gear, first cut: `cast heal`

Status: design, approved in conversation 2026-10-01. Not yet implemented.

## Goal

LEVELS.md, "Tabled": **abilities from gear** -- gear decides what you can *do*, as well
as how strong you are. It is the first thing the Barrow-King fight needs that doesn't
exist anywhere: until somebody can heal, the tank/healer/striker group is a label.

This spec builds the ability system and exactly one ability, `heal`. The system has to
take the ones coming after it without a redesign:

- **offensive** (a mace of smackdown: a big hit on a mob),
- **defensive**,
- **informative** (identify, and the like),
- **recall**, as a trinket: always available to a wizard, granted to anyone else only
  by gear. Today `recall` is a free command everybody has, and both the smoke bot and
  the inhabitant bots use it, so gating it is its own spec.

Success looks like: equip the sprig censer, `cast heal bob` mid-fight, and Bob gets
health back; take the censer off and you can't; the numbers are in a rules file; and a
typo in content fails startup rather than a cast.

## Standing rules (unchanged, and what this design leans on)

- **The gear is the truth.** Nothing on `player.Player` or `player.Record` says "can
  heal". Like role and power, what you can cast is derived from the slots on every read.
- **Nothing mechanical branches on a role.** A Healer label grants nothing; a censer does.
- **Go is the engine.** What an ability does is Go. Content says which item grants which
  ability, and tunes its numbers.

## Decisions

| Question | Decision |
|---|---|
| What limits casting | **Mana and a cooldown**, both. Mana is the budget; the cooldown stops spamming. |
| Max mana | **Flat, 100**, like max health today. Gear decides *what* you cast, not how much. |
| Mana regen | **Always**, in a fight or out -- unlike health. The healer is busy the whole fight. |
| Heal targets | **Yourself, or a player in your room.** Never a mob. |
| Verb | **`cast <ability> [target]`**, one command for every ability. |
| Definition | **A rules file plus Go effects** (below). Not all-Go, not Lua. |
| Wasted casts | **A cast always spends** its mana and cooldown, even healing someone unhurt. |
| Two items grant one ability | **The highest-power one** is the power the ability is cast at. |

## Data model

### `rules.Ability`, from `content/rules/abilities.json`

```json
[
  {
    "id": "heal",
    "name": "heal",
    "mana": 20,
    "cooldown": "10s",
    "target": "friend",
    "amount": { "base": 10, "per_power": 2 }
  }
]
```

- `id` is what objects and the `cast` command name. `name` is what the player reads.
- `mana` is the cost; `cooldown` a Go duration string, parsed at load.
- `target` is one of `self`, `friend` (you, or a player in your room), `foe` (a mob in
  your room), `none`. Heal is `friend`; the others exist for the abilities after it. The
  handler resolves the target once, by kind -- effects never parse target strings.
- `amount` is heal's own parameter. Each effect reads its own parameters from its entry;
  future abilities add their own keys (as optional fields on `rules.Ability`).

Loaded into `rules.Catalog.Abilities`, keyed on id. Absent file means no abilities, like
durability and economy. **Startup fails** on a duplicate id, a negative `mana`, a
negative or unparseable `cooldown`, or an unknown `target`.

### Granting

- `"abilities": ["heal"]` in objects.json becomes `object.Definition.Abilities []string`,
  assigned by the loader as `RoleWeights` is. **An id not in the catalog fails the load.**
- Only **equipped, unbroken** gear grants: a broken censer stops healing, as broken
  armor stops counting toward AC.
- `Equipment.Abilities()` answers each granted ability once, with the **highest power**
  among the items granting it and that item (for display). Order: catalog declaration
  order, so the listing is stable.

### Mana, on the player

- `curMana`, `maxMana` beside `curHealth`/`maxHealth`; max is 100 for everyone.
- Saved: `CurMana *int` on `player.Record` (and `mongostore.playerDoc`). A pointer for
  the durability reason: a record from before mana reads as full, not empty.
- `Player.SpendMana(n)`, `RestoreMana(n)` (capped), `CurrentMana()`, `MaxMana()`.

### Cooldowns, on the player

- `map[string]time.Time`: when each ability is ready again. **In memory only, never
  saved.** A quit resets it, which is harmless: logging back in takes longer than any
  cooldown.
- Time comes from a new `World.now func() time.Time` (`time.Now` by default), so tests
  move the clock rather than sleep. The world has none today: pulse code takes `now` as
  an argument (`decayFloors(now)`), which a handler can't. Only cooldowns use it in this
  spec; moving the other `time.Now()` calls onto it is not part of this work.

### Regen

`World.Regenerate` adds mana to **every** player, fighting or not:
`rules.ManaRegenAmount(max)`, a placeholder beside `rules.RegenAmount` -- 5% of max, at
least 1, per regen pulse (5s). Add its row to the LEVELS.md tuning table.

### Prompt

`event.Prompt` gains `CurrentMana, MaxMana`. It renders `<97/100hp 80/100m> `. Update:
`telnet/render.go`, `ansi_test.go`, and the bot's patterns (`bot/client.go` `promptRe`,
`bot/smoke.go` `lineStart`/`prompt` and the `get` expectation, and the bot test
transcripts). The bot reads health from the prompt; it keeps doing that.

## The cast

### Command and parse

`command.Cast{Ability, Target string}`, verb `cast`, alias `c`. `parse.go` splits off the
first word as the ability and passes the rest as `Target`, untouched -- splitting a word
off is not parsing a target; `world` still owns target grammar.

### `world/h_cast.go`

Checks, in order, each a `msg.Fail`:

1. no ability given -> `NoTarget` -- "Cast what?"
2. not in the catalog -> **`UnknownAbility`** -- "There's no such spell."
3. no equipped, unbroken item grants it -> **`NotGranted`** -- "Nothing you're wearing
   lets you cast heal." Says *why*, which teaches the gear rule.
4. on cooldown -> **`NotReady`** -- "You can't cast heal again yet."
5. resolve the target by kind. `friend`: empty is yourself; otherwise a player in this
   room by `player.NameKey`. A miss is `TargetNotFound`.
6. not enough mana -> **`NotEnoughMana`** -- "You don't have enough mana."

Then: spend the mana, start the cooldown, run the effect. **A cast that got this far
always spends**, whatever the effect finds.

`resultcode.go` gains the four new codes (verb-keyed as usual where the wording needs it).

### Effects, `world/abilities.go`

```go
type cast struct {
    caster  *player.Player
    target  *player.Player // nil for kinds without one; grows a mob field for foe
    ability rules.Ability
    power   int            // of the granting item
}

type effect func(w *World, c cast)

var effects = map[string]effect{
    "heal": healEffect,
}
```

**`world.New` fails** if any catalog ability has no entry in `effects` -- the other half
of the loader's check. Between them, content can't name an ability that does nothing,
and an object can't grant one that doesn't exist.

### Heal

- amount = `base + per_power * power`; `RestoreHealth` caps at max. Power 1 heals 12.
- Allowed **mid-fight**, for caster and target alike. It neither starts nor joins a
  fight. No threat, because threat doesn't exist yet.
- Healing someone unhurt is allowed and wasted: the event goes out with what was
  actually restored, possibly 0.

### Event

`event.Healed{Actor, Target string, Amount int}`, sent to the room -- one event per
thing that happened, rendered per viewer by comparing to `self`:

| Viewer | Text |
|---|---|
| caster, self | You heal yourself. (+12) |
| caster, other | You heal Bob. (+12) |
| caster, Amount 0 | You heal Bob, but Bob wasn't hurt. |
| target | Alice heals you. (+12) |
| bystander | Alice heals Bob. |

Each future ability gets its own event (`Smote`, ...). No generic `Cast` event.

### `abilities`

`command.Abilities`, `world/h_abilities.go`: lists what your gear grants right now, one
line each -- name, mana, cooldown, the granting item and its power, and `ready` or the
seconds left. Read-only, like `role`. Nothing granted: "Nothing you're wearing grants
any abilities."

## Content

- `sprig_censer` (hollowfield) and `healers_censer` (sample) get `"abilities": ["heal"]`
  -- the sprig is a newbie's real path to a heal.
- `bone_charm` (barrow) grants heal as well: a better heal from the same ability.
- `content/rules/abilities.json` as above. Every number is a placeholder until play.

## Out of scope

Wizard bypass; recall as an ability (and gating the `recall` command); offensive,
defensive and informative abilities; threat; group/party health display; max mana from
gear. `target` kinds and the `effects` map are where those plug in.

## Testing

TDD, piece by piece:

- `rules`: abilities.json loads; each refusal above fails.
- loader: an object granting an unknown ability fails.
- `world.New`: a catalog ability with no effect fails.
- `Equipment.Abilities()`: equipped only; broken excluded; highest power wins; stable order.
- player/record: mana round-trips; a record without `CurMana` loads full.
- regen: mana comes back in a fight; health still doesn't.
- `h_cast_test.go`: each refusal in order; self and other; mid-fight; unhurt target
  still spends; cooldown ready again once the clock moves.
- `telnet/render_test.go`: `cast` and `abilities`; `help.go` entries for both
  (`help_test.go` checks the verbs parse); `ansi_test.go` for the prompt.
- `go test ./bot` green with the new prompt.

## Implementation order

Each step green on `make check`, and its own commit on master.

1. `rules.Ability` + abilities.json loading and refusals.
2. `Definition.Abilities` from objects.json + `Equipment.Abilities()`.
3. Mana on the player and the record; mana regen.
4. The prompt, renderer and bots.
5. `cast` + heal + the effects check in `world.New`.
6. `abilities`.
7. Content: abilities.json, the censers, the charm.
8. Docs: CLAUDE.md "Abilities" (beside "Lineage and Role"); LEVELS.md moves abilities
   from Tabled into Phases; the site guide paragraph, held until release.
