# Overnight, 2026-10-06

## Start here (written for 07:30)

Everything below is on PR #36's branch, CI green, nothing released or deployed.

**Wants you:**
1. **A hotfix for v0.10.0?** Eleven fixes are for bugs live today. Seven of them are
   ready as a patch series on the tag, **`docs/hotfix-v0.10.1/`** -- its README says
   how to apply them (`git am`) and what each is; checked: they apply to a fresh
   v0.10.0 and every test passes. The rest are bigger or conflict with the old bots;
   they ride the next release.
2. **Try Mudlet** (the PR's add-on; merging publishes its site section). If the map
   misbehaves: close Mudlet's own map window, uninstall its `generic_mapper`. Rooms now
   come with grid coordinates from the server, so a recall shouldn't stack rooms.
3. **Calls I made that are yours to undo:** potions (a small version: `quaff`, a
   healing draught at the General Store); 10 new characters a day per address; 200
   connections at most; a 1s pause before a name is asked again and 2s after a wrong
   password; "nothing" became a reserved name; keys are `noSell`.
4. **Still open from before:** a fleeing bandit takes its loot with it; the
   placeholder numbers (position regen, full-moon effects, potion price/cooldown).

**What the night was:** the CircleMUD list you picked (socials, wizard tools, reports,
positions, small commands, track, the moon), potions, then reviews -- fifteen of
them, until every major part of the server had had one, each finding checked before
fixing: the night's own code, the morning's, the first night's, security, saving,
Mudlet, core mechanics, the bots, my own fixes, rendering, the item commands, the small
commands and loader, and the script engine. Some fifty real bugs fixed; the worst are
in the PR's "In production today" and "Fixes found along the way". tintin++ was tried
for real and works.


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

Fixed later in the night: after a *crash*, an item given or coins split between two
players could exist twice -- the receiver who quit was saved at once, the giver only
on the next interval. Drop, give, put, donate and split now save everyone in the
room at once; saves only queue, so it's cheap.

A security review of the connection and login layer (the injection fix held up):
- A store error mid-login hung the conversation for good, and five of them locked an
  address out. Now answered.
- One connection pipelining names kept the world goroutine doing store lookups.
  A name asked again now waits 1s; a wrong password 2s; bcrypt one per CPU.
- Logins must finish in 5 minutes; 200 connections at most; IPv6 counted by /64.
- Unicode format characters (right-to-left override) dropped from typed lines.
- Done since: the login lookup is off the world goroutine entirely (a mongo blip
  froze the whole game for up to 5s per login); the record is reloaded if the
  character logged out while the password was checked. Not done: a per-name lockout
  after failed passwords
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

### If you want a hotfix before this PR ships

Eleven of the night's fixes are for bugs in v0.10.0. Releasing is yours; this is only
which commits carry them, for a `hotfix/` branch off the release tag:
- `bd01955` -- typed lines cleaned, IAC doubled (telnet/conn.go). Self-contained.
- `2e59676` -- write-behind retries and distrusts failed writes (writebehind/).
  Self-contained apart from the docs.
- `78b5d45` -- only `world/zone_activity.go` and `spaces/zone.go`'s `HasPlayers` are
  the Barrow fix; the rest is the mill (not in v0.10.0).
- `516e354` -- only `server/gameserver.go`'s `recovering` answering a failed login is
  the hung-login fix; the pacing and limits came with it and could ride along.
Later in the night, two more fixes for v0.10.0 bugs (not tried on the tag):
- `42d9643` -- the login lookup off the world goroutine (a mongo blip froze the game
  per login). A bigger change to `server/`; would need the login tests reworked.
- `71e9c95` -- hand-overs save everyone in the room at once (crash duplicates).
  Small: `world/handlers.go` and `world/playersave.go`.
- `8d04177` -- a login whose connection hangs up midway leaves no ghost. v0.10.0 has
  the same hole in its bcrypt window; the fix there would be the same idea (note
  connections that log out with a hash away, drop what comes back for them) without
  the lookup changes it's written against here.
- `3f51f88` -- `parseTarget` refuses `-1.x`, `2.` and `0 coins`
  (`world/target_parser.go` only).
- `3ab69cb` -- `kill` takes the whole name (one line in `telnet/parse.go`; the rest of
  the commit is smaller fixes that can ride along or not).
- `45d7a57` -- the bots' patterns anchored, so a player can't knock one off by
  talking. `bot/` only (and "nothing" reserved in `world/names.go`); the bots ship in
  the same image, so it rides any release.

Tried in a scratch worktree off `v0.10.0` (not pushed): the first two cherry-pick
cleanly once the docs are dropped; the Barrow and hung-login parts (and their
tests, `world/zone_activity_test.go`, `server/recover_test.go`) go in by hand; and
`go vet` and the whole of `go test ./...` pass on the result.

Two more reviews, of the core mechanics and of the bots:
- Mechanics (none reachable with today's content): a player killed by one of two mobs
  in the death room could die twice in a round; negative dice (1d2-3) healed; aggro
  ignored nofight rooms.
- **Bots: any player could knock one off the server by talking.** Their patterns
  weren't anchored, so a say of "You are dead!" sent a bot home and one of recall's
  refusal disconnected it. Every pattern is anchored now, consider's to the prey's own
  name, and "nothing" is a reserved name (an emote by a Nothing would be the refusal).
- A hunter kept fighting and looting after a player walked in; it now looks when
  someone enters and leaves the ground to them.
- The smoke test can fail on a full-moon night (a wild dog catches it on the walk): it
  now says "attacked on the way", and deploy/README says that isn't a bad deploy.
  `TestKeepOut_safe` counts moonstruck mobs, and a wanderer's whole zone.
- An explorer now forgets an exit a room no longer shows (a door shut since), and a
  bot waiting out recall's cooldown stops when it's told to shut down.

A last review, of the fixes above:
- **A connection that hung up mid-login came back as a ghost:** the late answer put
  the character in the world with nobody there, and no Logout ever came, so the real
  player was refused as already playing until a restart. (Possible before tonight in
  the bcrypt window; the store lookup made the window wider.) Results for a
  connection that's gone are dropped now.
- A panic in a login's last step left the conversation waiting for good; an empty
  creation password was answered twice; bots timed out on "They aren't taking
  tells." and missed a player logging in on their ground.

A review of what players read (render, wrap, tables, prompts, GMCP, help):
- **`kill wild dog` attacked the wild boar:** kill kept only the first word, and
  "wild" finds whichever wild thing is listed first (bandit/bandit lookout and the
  Miller/millhand the same). It takes the whole name now, as consider always did.
- `commands` showed help instead of its list of verbs: help caught the word first.
- `'hello` and `:waves` (no space) were "Unknown request".
- `gold 1` said "1 coins appear"; tables counted bytes, so a "café" would misalign.

A review of the item commands (no duplicates or minted coins found; three bugs, all in
the target grammar, the first two in v0.10.0 too):
- `get -1.knife` (or any handler's `-1.x`) panicked: recovered, but a stack trace in the
  log per attempt -- a free log flood.
- `2.` with nothing after the dot acted on whatever was second: a half-typed `junk 2.`
  destroyed an item. `all.` the same, as everything.
- `put 0 coins in chest` put every coin in it.
All three are refused as parse errors now (`parseTarget`).

A review of the small commands and the loader:
- A one-room `followPath` panicked every mobile pulse, and stopped every mob after it
  from wandering or aggroing that pulse. The loader now wants two real rooms.
- A room, object or mob id used twice in a zone quietly replaced the first -- a
  copy-paste would have cut Temple Square off. Refused now.
- A dropped item with the same name hid a chest from `unlock`.

A review of the Lua script engine (every removal path forgets a mob's runs; caps
span waits; the real scripts behave) found two sandbox bugs:
- **One script could change string methods for every other**: each program's copy of
  `string` still carried `string.__index`, the one table all `("x"):upper()` calls
  go through. A bad script could have had the Barrow-King and the shopkeeper switched
  off. Strings now have a metatable no program can reach.
- `wait` inside an iterator or a metamethod didn't pause -- gopher-lua swallows that
  yield -- and the hook ran straight on. Now it's an error that says why.

Checks against the real thing:
- **tintin++** (2.02.20, installed here): a scripted session made a character with no
  typing -- its `#action`s fire on our GA-marked prompts -- and played; wrapping,
  color, password echo all right (ROADMAP Phase 7).
- **Mudlet** couldn't be fetched here (the network policy refuses GitHub's release
  downloads), so it's still yours to try.
- **Load test** after the night's changes: 100 bots at a person's pace, none dropped,
  `look` p99 22ms. At test pace (`-fast`), 19, 5 and 1 dropped across three runs --
  the 19 on the run with a quarter fewer steps, i.e. a busy machine; it was 1 before.

The hunter test that failed once under -race (10 kills, 1 looted): not reproduced in 26
race runs since, and the hunter now logs what an empty loot attempt saw, so if it comes
back it explains itself. Chasing it found two real problems, both fixed:
- **Who swings first in a round was random** -- `FightLedger.GetFights` ranged a map.
  It now returns fights in the order they began. (CI caught this through a test.)
- **The world could fail to build on a busy machine**: a script's top level ran under
  the 10ms a hook gets, and the shopkeeper's table-building once took longer under the
  race detector. Top levels now get `script.LoadTimeout` (500ms).
