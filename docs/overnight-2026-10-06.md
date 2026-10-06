# Overnight, 2026-10-06

Unattended work on `claude/watchmud-roadmap-development-2gbc8l` (PR #36), in order, until
the session's credit runs out. Each task is its own commit with `make check` green,
pushed when it lands, so whatever is ticked here is on the branch. Nothing touches
production, and nothing here is released.

- [x] **1. Review PR #36's own diff** for bugs before anyone merges it (doors, chests,
  put, bags, the status tables, the mill) and fix what's real.
- [x] **2. `give <item|coins> to <player>`**: `object.Move` between two inventories,
  in the same room; worn things stay on, as with `drop` and `put`.
- [x] **3. `examine <item>`**: what one thing is -- power, condition, armor, the
  abilities it grants, a bag's room -- for something carried, worn or on the floor.
- [x] **4. A script hook for speech, `on_hear(me, speaker, text)`**, and the
  hedge-witch answering `say heal` with a line (words only: healing a player from a
  script would be a new Go action, raised rather than decided).
- [x] **5. Small cleanup**: `world/settings.go`'s lone `VERBOSE_LOGGING` const.
- [x] **6. Stretch: split `spaces.Room` into definition and instance** (ROADMAP "Known
  problems"): a spec first, then the refactor in steps, each green. *Spec only -- see
  notes.*

- [x] **7. FAQ answers for the new commands** (give, look/examine, bags), so the
  socialite documents them for the newest players.
- [x] **9. Draft the site guide's rows** for the release (`docs/site-next-release.md`):
  `site/` publishes on reaching master, so it can't go in the PR.
- [x] **10. Enforce unique room names** in content: the bots navigate by them, and
  ROADMAP noted it was "true today, not enforced".
- [x] **8. Hunt the two flaky tests** seen once each earlier (a bot game test,
  `TestMoveWhileFighting`): run the suite many times, root-cause what fails.

- [x] **11. `drop 5 coins`** said "You aren't carrying that." (a TODO in `h_drop.go`);
  it now says coins stay in the purse and to give them instead.

Feel and tuning calls met along the way are noted below rather than decided.

## Notes

- **1.** Two fixes, both in zone resets. A missing `instance_max` meant "another every
  reset" for an object and "never" for a mob; every instruction in content sets it, so
  nothing was wrong yet, and the loader now refuses one under 1. And a reset counted
  objects by definition id alone, so another zone's `key` filled this zone's quota; it
  counts the definition now. Read and found sound: put, get-from, findContainer, the
  door handler and its lock errors, bag persistence (writebehind compares records with
  `reflect.DeepEqual`, so a change inside a bag is saved).
- **2.** `give` is put towards a player. To fit `help` on one screen (45 lines, a test)
  `inventory` and `equipment` now share a line. `give coins to bob` with no number gives
  every coin, as `put coins in chest` does -- consistent, but easy to fat-finger;
  requiring a number is a one-line change if that reads as a trap. The site guide
  isn't touched (it waits for a release).
- **3.** Found that `look <thing>` was in `help` but did nothing -- the handler showed
  the room whatever you named. It now looks at a player, a mob, or an object (carried,
  worn, then the floor), with `examine`/`exa` as aliases. A mob shows how hurt it looks
  in words (perfect / slightly hurt / wounded / badly hurt / near death at 100/75/50/25)
  and nothing assess gives; the bands are a feel call, easy to move.
- **4.** `on_hear(me, speaker, said)`, with `said.words` as a set because scripts
  have no `string.find`. Mobs don't hear mobs. The hedge-witch answers heal / healing /
  hurt / wounded / mend with two placeholder lines -- the second tells players she
  carries a censer they'd have to take off her, which is true (she drops it, 15%) but
  is a nudge to kill her: reword it if that's not wanted. No new Go action: she talks,
  she doesn't heal.
- **5.** `VERBOSE_LOGGING` gone; ping logs at trace. The stdlib-`log` half of the
  ROADMAP item had already gone everywhere but `cmd/watchmud-bots`.
- **6.** Spec, not code: `docs/superpowers/specs/2026-10-06-room-definition-split-design.md`.
  Looking closer, `Zone` conflates content and live state too, and `loader.Content`
  isn't immutable (the world plays in its rooms) -- but the writers are loader-only,
  so nothing is broken. ~150 call sites; a design to agree before doing, with a
  recommendation to wait until something needs it.
- **7.** Two socialite topics: give (before shops, so "give coins" isn't a shop
  question) and examining things (`look <name>`). Bags got theirs with the bags commit.
- **9.** The live site already says `look goose` looks at one thing; in v0.10.0 it
  didn't. True from the next release; flagged in the draft and in working.md.
- **8.** Not reproduced: 15 full-suite runs, 5 under `-race` (world, bot, telnet,
  server) and 5 at `-cpu=1`, all green. The move test already reports what it was
  fighting if it fails again, which is the lead to follow then.

## Decided next morning

- Room/zone split: leave it for now (the spec says so).
- The hedge-witch must not encourage attacking her: her second line now points at
  gear that grants heal and `abilities`.
- `give coins to bob` needs a number (`NO_VALUE`, "How many? Give 20 coins to
  someone."); `put coins in chest` still takes them all.
- Mob health words stay as they are.
- Mudlet: to try tomorrow.
- CI caught what `make check` can't: `TestStore_documentShape` (real-mongo only)
  counted two inventory items, and the bags commit added a third to the shared test
  record. It now counts the record's own, and checks the bag survives a real round trip.

## Second night (owner asleep from about 08:40 UTC; back 07:30 PDT)

Picked by the owner from the CircleMUD comparison: 1 and 2 (socials, talking), 4 (wizard
tools), 5 (reports), 3 (positions), 6 (small commands), 7 (track); not 8, 9 (mudmail --
Discord, later), 10; and, low priority, the moon. All done, each its own commit:

- [x] Socials (53, our own words), emote, reply, whisper/ask (mobs too), toggle
- [x] Wizard tools: goto, transfer, purge, zreset, echo/gecho, users, mute, freeze
- [x] Player reports: bug, idea, typo -> log, mongo, wizards' `reports`
- [x] Positions: sit, rest, sleep, stand, wake
- [x] wimpy, split, where, time, commands; hit, hold, score
- [x] track <mob>
- [x] The moon over Seattle: full-moon nights (moonstruck wild dogs, luckier loot,
  "Full moon tonight. Be careful.")

Found along the way, fixed:
- A whole set of failure texts (bags, groups, ooc, donate) had been filed under the
  per-verb map with no verb, so players saw "You can't do that." A test now reads
  `event/result.go` and fails on any code without words. `CANT_FLEE` had none either.
- The moon reads its own clock, pinned in tests: otherwise every full-moon night would
  have changed what the tests saw.

Calls made without the owner (cheap to change):
- Positions regen 150/200/300%; waking stands you up (Circle sits you up).
- A sleeping player can wake, stand, check stat/inventory/equipment/abilities/group/
  toggle/who, and quit -- nothing else.
- Mute and freeze are a second `mute`/`freeze` to undo; wizards can't be moderated.
- Reports: 500 characters, the latest 50 kept in memory.
- Full-moon effects: wild dogs (power 2) moonstruck; loot bump chance doubled; the
  night is 6 pm to 6 am Seattle time.
- `give`'s, `whisper`'s and `ask`'s room lines say only that something was said.

After that, on my own judgement (each its own commit):
- Bots rest sitting down (`rest`) and `stand` before walking on, so they look like
  players and heal at the resting rate.
- Potions -- your "eh, lets see" (8). Kept small: `"quaff": "heal"` on an object makes
  it a potion; `quaff`/`drink` runs the ability on yourself at the potion's power, no
  mana, and it's gone. One potion per 10s, shared. The General Store sells a healing
  draught (power 2: 14 health, 20 coins). Easy to take out if it isn't wanted: one
  object, one shop line, one handler.

Then a review of the night's code (two reviewers, each finding checked before fixing):
- **Telnet injection** (the worst; older than tonight, wider since): a doubled IAC
  typed into a line reached the world as a 0xFF byte, so `emote` or `say` could send
  the room `IAC WILL ECHO` (hiding what they type) or an escape that clears screens.
  Lines are cleaned on the way in (`telnet.clean`) and 0xFF doubled on the way out.
- `goto` out of a fight left a fight that could never swing or end.
- `split` told bystanders "you get 15 coins".
- `bug`/`idea`/`typo` had no rate limit: now one per player per 10s.
- On a full-moon night wild dogs stopped wandering; and "full moon tonight" could
  be wrong by dark. A night is now full or not as a whole (the phase at midnight).
- Assist dragged sleeping group members into fights.
- A muted player's typo said "muted"; muting a wizard said "remove it first".

A second review, of the morning's code (groups, give, bags, janitor, scripts):
- A frozen player still walked after their leader and joined the group's fights.
- A bag whose definition leaves content took its contents with it at login.
- The janitor swept a dropped bag and everything in it.
- A crumbling summon would have taken what it carried (none carry today).
- Wimpy could run from a fight the bandit's script had already ended.

Not fixed, for you: after a *crash* (not a clean shutdown), an item given, or coins
split, between two players can exist twice -- the receiver who quits is saved at
once, the giver only on the next interval. Saving both sides of a transfer would
close it; it didn't seem worth the writes without a crash to point at.

A security review of the connection and login layer (the injection fix held up):
- A store error mid-login hung the conversation for good, and five of them locked an
  address out. Now answered.
- One connection pipelining names kept the world goroutine doing store lookups.
  A name asked again now waits 1s; a wrong password 2s; bcrypt one per CPU.
- Logins must finish in 5 minutes; 200 connections at most; IPv6 counted by /64.
- Unicode format characters (right-to-left override) dropped from typed lines.
- Not done: moving the login lookup off the world goroutine entirely (the pause makes
  it one lookup a second a connection); a per-name lockout after failed passwords
  (it would let anyone lock a player out). Creations are capped since: 10 a day per
  address.
- Seen once since, not reproduced in 4 more runs: `TestAdventurer_answersATell`
  timed out ("Wren never came online") under the full race suite.

A review of the first night's code (doors, chests, the mill, abilities):
- **A trap in the mill:** every reset re-locked the grate, the wheel pit's only exit,
  with the Drowned Miller respawned beside whoever was in there. Without the key,
  flee and recall both refused, and dying was the only way out. A door with a player
  on either side is now left alone by resets.
- "noPlayers" zones (the Barrow) reset with players in them; they wait now.
- The strongbox key came back every reset and sold for ~16 coins: keys are `noSell`.
- A stun outlasted its fight when the stunner fled or fell.

A review of saving (write-behind, mongo, the record), which found every field
round-trips and nothing aliased, and two real bugs, both in production today:
- **A failed save waited for some other save to be retried.** The last player out
  quitting during a mongo blip sat in memory only, for hours if nobody came; and
  Close tried once. Now retried on its own (1s doubling to 30s), five times at close.
- **A write reported failed that had landed** could make a later, real change be
  skipped as "identical" -- remove a sword, wield it again, and the store kept the
  removal. A failed write now forgets what was last written for that player.

Before you try Mudlet (a review against Mudlet's own source; its wiki was out of
reach from here): nothing found that would stop it loading, but --
- **Rooms you jumped into stacked up:** a recall, login or death into a room with no
  mapped neighbour went to the area's origin, often on top of another. The server
  now lays every zone out on a grid at load and sends each room's place in
  Room.Info (`"grid"`); the script uses it. Every exit in the content fits the grid
  exactly, and a test keeps it that way.
- Uninstalling now takes the bars, map and handlers away (`sysUninstall`).
- The site's Mudlet section now says: close Mudlet's own map window if the map box
  stays empty, and uninstall Mudlet's `generic_mapper` if the map jumps about -- it
  reads our `[ Exits: ]` line and recentres the map itself. Neither checked live.

The hunter test that failed once under -race (10 kills, 1 looted): not reproduced in 26
race runs since, and the hunter now logs what an empty loot attempt saw, so if it comes
back it explains itself. Chasing it found two real problems, both fixed:
- **Who swings first in a round was random** -- `FightLedger.GetFights` ranged a map.
  It now returns fights in the order they began. (CI caught this through a test.)
- **The world could fail to build on a busy machine**: a script's top level ran under
  the 10ms a hook gets, and the shopkeeper's table-building once took longer under the
  race detector. Top levels now get `script.LoadTimeout` (500ms).
