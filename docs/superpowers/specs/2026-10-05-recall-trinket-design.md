# Recall as a trinket

Status: design, approved in conversation 2026-10-05. Not yet implemented.

## Goal

LEVELS.md phase 5's last named ability: **recall comes from gear.** Wizards always have
it; everyone else needs something worn that grants it. It makes recall a slot a player
chooses to spend -- the neck a temple token sits on is the neck a bone charm can't.

Success: a new character can recall from minute one; an existing one -- player or bot --
gets the token once at their next login with no hand work; a player who takes it off
can't recall until they put it back; and the smoke bot and the inhabitants keep working.

## Decisions (from the conversation)

- New characters start with the token **worn** (neck).
- Existing characters get it **once, at login**: worn if the neck is free, else in the
  pack, with a line saying which.
- **No mana, 60s cooldown**, refused mid-fight as today.
- The command stays **`recall`**; `cast recall` works too.

## Content

- `wrathrock/temple_token`: "a temple token", neck, category OTHER, `"durability": 0`
  (indestructible: dying wears everything worn, and the one item that gets a dead player
  home must not be the one that breaks), `"abilities": ["recall"]`.
- The General Store sells it at power 1, so a lost one can be replaced.
- `starting_gear.json`: `{ "zone": "wrathrock", "object": "temple_token", "equip": true,
  "power": 1, "backfill": true }`.
- `abilities.json`: `recall` -- target `none`, mana 0, cooldown `60s`,
  `"not_in_fight": true`, `"wizards": true`.

## Engine

**Two ability flags**, data not special cases:

- `NotInFight` (`not_in_fight`): `handleCast` refuses `IN_A_FIGHT` before spending,
  after `NOT_READY`.
- `Wizards` (`wizards`): a wizard needs no granting gear and has no cooldown.

**`none` target** in `castTarget`: nothing to resolve. Recall is its first user.

**`recallEffect`**: what `handleRecall` did -- `movePlayerMagically` to the start room
and the room description. Output unchanged.

**`handleRecall`** becomes the `recall` ability cast through the same path as
`handleCast` (shared function taking the ability id and target), so the verb on a
failure is `recall` and gets its own wording:

- `recall/NOT_GRANTED`: "Nothing you're wearing lets you recall -- a temple token does;
  the General Store sells them."
- `recall/NOT_READY`: "You can't recall again yet."

**Backfill.** `StartingGearItem.Backfill` (`"backfill"`). `player.Record.Backfilled
[]string` ("zone/object"), on `Player` and in `mongostore`'s document. At creation
`GiveStartingGear` marks every backfill item received. At login, `World.Arrive` gives
each backfill item a character hasn't received -- worn if its slot is free, else in the
pack -- marks it, and tells them with `event.Received{Item, Worn}`: "You find a temple
token around your neck." / "You find a temple token in your pack. Wear it to recall."
The next timed save keeps it; it is creation-only in the sense that it happens once.

## Bots

- **Smoke**: the first recall only if it isn't already in Temple Square (it normally
  logs in there, having recalled at the end of its last run), so a normal run recalls
  once and never meets the cooldown.
- **Adventurer**: a recall answered "You can't recall again yet." waits 10s and tries
  again, up to the cooldown.

## Testing

rules (flags parse), loader (real content: the token, the kit), world (recall granted /
not granted / wizard / cooldown / in a fight / `cast recall`; backfill worn, pack,
once, never for a new character), player/mongostore (Backfilled round trip), telnet
(render cases), bot (smoke and adventurer tests against the real content).

## Docs

CLAUDE.md "Abilities" (recall, the two flags, `none`, backfill); LEVELS.md (recall
done). The site guide waits for the release.
